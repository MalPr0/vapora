package punch

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// inviteVersion is the first byte of a room invite. It exists so a token from
// another version fails while it is still a string, before anything touches the
// network.
const inviteVersion byte = 1

var (
	ErrInviteVersion = errors.New("punch: invite is from another version")
	ErrNotRoomInvite = errors.New("punch: not a room invite")
)

// RoomInvite is what a member hands out. It carries the endpoint to punch at,
// the room secret, and the public key of whoever issued it.
//
// The key is not decoration. Without it the room secret would be enough to
// answer a newcomer's hello, so anybody who saw the invite could stand between
// the newcomer and the room. Pinning the issuer means the welcome has to come
// from the one who wrote the line.
type RoomInvite struct {
	Endpoint *net.UDPAddr
	Secret   Secret
	Host     PublicKey
}

func (i RoomInvite) Token() string {
	blob := make([]byte, 0, 1+secretBytes+PublicKeySize)
	blob = append(blob, inviteVersion)
	blob = append(blob, i.Secret...)
	blob = append(blob, i.Host[:]...)

	return i.Endpoint.String() + secretSeparator + inviteEncoding.EncodeToString(blob)
}

func (i RoomInvite) Command(command string) string {
	return fmt.Sprintf("%s %s", command, i.Token())
}

// ParseRoomInvite accepts a bare token or a whole pasted command, so the user
// never has to trim anything by hand.
func ParseRoomInvite(line string) (RoomInvite, error) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return RoomInvite{}, fmt.Errorf("%w: %q is empty", ErrNotRoomInvite, line)
	}

	endpointPart, blobPart, found := strings.Cut(joinToken(fields), secretSeparator)
	if !found {
		return RoomInvite{}, fmt.Errorf("%w: %q carries no room", ErrNotRoomInvite, fields[len(fields)-1])
	}

	endpoint, err := net.ResolveUDPAddr("udp4", endpointPart)
	if err != nil || endpoint.Port == 0 {
		return RoomInvite{}, fmt.Errorf("%w: %q is not a host:port endpoint", ErrNotRoomInvite, endpointPart)
	}

	blob, err := inviteEncoding.DecodeString(blobPart)
	if err != nil {
		return RoomInvite{}, fmt.Errorf("%w: %w", ErrNotRoomInvite, err)
	}
	if len(blob) == secretBytes {
		// The one to one invite is exactly a bare secret, so saying which tool
		// this belongs to beats a decoding error nobody can act on.
		return RoomInvite{}, fmt.Errorf("%w: that is a `vapora punch` invite, not a room", ErrNotRoomInvite)
	}
	if len(blob) != 1+secretBytes+PublicKeySize {
		return RoomInvite{}, fmt.Errorf("%w: %d bytes", ErrNotRoomInvite, len(blob))
	}
	if blob[0] != inviteVersion {
		return RoomInvite{}, fmt.Errorf("%w: version %d, this build speaks %d", ErrInviteVersion, blob[0], inviteVersion)
	}

	invite := RoomInvite{Endpoint: endpoint, Secret: Secret(blob[1 : 1+secretBytes])}
	copy(invite.Host[:], blob[1+secretBytes:])
	if invite.Host.isZero() {
		return RoomInvite{}, fmt.Errorf("%w: no host key", ErrNotRoomInvite)
	}
	return invite, nil
}
