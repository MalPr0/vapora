package punch

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

// entryBytes is a public key, an IPv4 address and a port.
const entryBytes = PublicKeySize + 4 + 2

var ErrRosterTooLarge = errors.New("punch: roster names more members than a room holds")

// Entry is one member as somebody else sees them.
//
// An entry is a suggestion, never a proof. It says where to try punching; only
// a frame that opens under the pair key says anybody is actually there.
type Entry struct {
	Key  PublicKey
	Addr *net.UDPAddr
}

type Roster []Entry

func (r Roster) Marshal() string {
	blob := make([]byte, 0, 1+len(r)*entryBytes)
	blob = append(blob, byte(len(r)))

	for _, entry := range r {
		blob = append(blob, entry.Key[:]...)
		if ipv4 := entry.Addr.IP.To4(); ipv4 != nil {
			blob = append(blob, ipv4...)
		} else {
			blob = append(blob, 0, 0, 0, 0)
		}
		blob = binary.BigEndian.AppendUint16(blob, uint16(entry.Addr.Port))
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
		entry.Addr = &net.UDPAddr{
			IP:   net.IPv4(record[32], record[33], record[34], record[35]),
			Port: int(binary.BigEndian.Uint16(record[36:38])),
		}
		if entry.Addr.Port == 0 {
			return nil, fmt.Errorf("%w: entry %d has no port", ErrMalformedRoster, i)
		}
		roster = append(roster, entry)
	}
	return roster, nil
}

var ErrMalformedRoster = errors.New("punch: malformed roster")
