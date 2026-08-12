package punch

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

// entryBytes is a public key and two candidates, each an IPv4 address and a
// port: where the world sees this member, and where their own network does.
//
// Two candidates rather than one because a single address cannot describe
// somebody behind the same router as you. Their public address is unreachable
// from beside them, and their local one is meaningless from anywhere else.
const (
	candidateBytes = 4 + 2
	entryBytes     = PublicKeySize + 2*candidateBytes
)

// ErrRosterTooLarge means somebody announced more members than a room holds.
// It is enforced while parsing rather than afterwards, so a hostile roster
// cannot make this side allocate on its say-so.
var ErrRosterTooLarge = errors.New("punch: roster names more members than a room holds")

// Entry is one member as somebody else sees them.
//
// An entry is a suggestion, never a proof. It says where to try punching; only
// a frame that opens under the pair key says anybody is actually there.
type Entry struct {
	Key PublicKey
	// Addr is the address the outside world sees, which is the one that works
	// for everybody except somebody sharing a router with this member.
	Addr *net.UDPAddr
	// Local is where this member sits on its own network, and is nil when it
	// could not be worked out. It is meaningless from outside that network and
	// is the only thing that works from inside it.
	Local *net.UDPAddr
}

// Candidates is every address worth punching at, best first.
func (e Entry) Candidates() []*net.UDPAddr {
	var addrs []*net.UDPAddr
	if e.Addr != nil {
		addrs = append(addrs, e.Addr)
	}
	if e.Local != nil && !sameEndpoint(e.Local, e.Addr) {
		addrs = append(addrs, e.Local)
	}
	return addrs
}

// Roster is who somebody says is present. It is gossip: every entry is a
// suggestion about where to try, and only cryptography settles who is real.
type Roster []Entry

// Marshal packs a roster into a padded payload of fixed size entries.
func (r Roster) Marshal() string {
	blob := make([]byte, 0, 1+len(r)*entryBytes)
	blob = append(blob, byte(len(r)))

	for _, entry := range r {
		blob = append(blob, entry.Key[:]...)
		blob = appendCandidate(blob, entry.Addr)
		blob = appendCandidate(blob, entry.Local)
	}
	return padded(blob)
}

// ParseRoster reads a roster and refuses one that names more members than a
// room can hold. The cap is enforced here rather than after the fact, so a
// roster from anywhere cannot make this side allocate on its say so.
func ParseRoster(payload string, max int) (Roster, error) {
	if len(payload) < 1 {
		return nil, fmt.Errorf("%w: empty", ErrMalformedRoster)
	}

	count := int(payload[0])
	if count > max {
		return nil, fmt.Errorf("%w: %d entries, this room holds %d", ErrRosterTooLarge, count, max)
	}
	if len(payload) < 1+count*entryBytes {
		return nil, fmt.Errorf("%w: %d entries announced, %d bytes carried", ErrMalformedRoster, count, len(payload))
	}

	roster := make(Roster, 0, count)
	blob := []byte(payload[1:])
	for i := 0; i < count; i++ {
		record := blob[i*entryBytes : (i+1)*entryBytes]

		var entry Entry
		copy(entry.Key[:], record[:PublicKeySize])
		if entry.Key.isZero() {
			return nil, fmt.Errorf("%w: entry %d has no key", ErrMalformedRoster, i)
		}

		entry.Addr = readCandidate(record[PublicKeySize : PublicKeySize+candidateBytes])
		entry.Local = readCandidate(record[PublicKeySize+candidateBytes:])
		if entry.Addr == nil {
			return nil, fmt.Errorf("%w: entry %d has nowhere to be reached", ErrMalformedRoster, i)
		}
		roster = append(roster, entry)
	}
	return roster, nil
}

// appendCandidate writes one address, or six zero bytes when there is none.
// An absent candidate has to occupy its space so entries stay a fixed size.
func appendCandidate(blob []byte, addr *net.UDPAddr) []byte {
	if addr == nil {
		return append(blob, 0, 0, 0, 0, 0, 0)
	}
	if ipv4 := addr.IP.To4(); ipv4 != nil {
		blob = append(blob, ipv4...)
	} else {
		blob = append(blob, 0, 0, 0, 0)
	}
	return binary.BigEndian.AppendUint16(blob, uint16(addr.Port))
}

// readCandidate reads one address back, and reports nothing for the zeroes an
// absent one leaves behind.
func readCandidate(record []byte) *net.UDPAddr {
	port := binary.BigEndian.Uint16(record[4:6])
	if port == 0 {
		return nil
	}
	ip := net.IPv4(record[0], record[1], record[2], record[3])
	if ip.IsUnspecified() {
		return nil
	}
	return &net.UDPAddr{IP: ip, Port: int(port)}
}

// ErrMalformedRoster covers a roster that does not decode: a truncated entry,
// a member with no key, or one with nowhere to be reached.
var ErrMalformedRoster = errors.New("punch: malformed roster")
