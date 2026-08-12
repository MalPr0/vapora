package punch

import (
	"bytes"
	"net"
	"strings"
	"testing"
)

// A room invite is wider than a terminal, so it reaches the other side split
// across lines as often as not. A paste that decodes to half a key is the worst
// failure this tool has: it looks copyable and it is not proofreadable.
func TestRoomInviteSurvivesBeingWrapped(t *testing.T) {
	host, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}

	invite := RoomInvite{
		Endpoint: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 41001},
		Secret:   secret,
		Host:     host.Public(),
	}
	whole := invite.Command("room")

	// A terminal is at least 40 columns wide, which is wider than the longest
	// endpoint, so the host:port always survives in one piece and only the blob
	// is split. Narrower than that is not a terminal anyone shares from.
	for _, width := range []int{40, 78, 80, 200} {
		var wrapped []string
		for runes := []rune(whole); len(runes) > 0; {
			cut := width
			if cut > len(runes) {
				cut = len(runes)
			}
			wrapped = append(wrapped, string(runes[:cut]))
			runes = runes[cut:]
		}

		for _, glue := range []string{"\n", " ", "\r\n", "\n  "} {
			got, err := ParseRoomInvite(strings.Join(wrapped, glue))
			if err != nil {
				t.Fatalf("wrapped at %d joined by %q: %v", width, glue, err)
			}
			if !bytes.Equal(got.Secret, invite.Secret) || got.Host != invite.Host ||
				got.Endpoint.String() != invite.Endpoint.String() {
				t.Fatalf("wrapped at %d came back as a different invite", width)
			}
		}
	}
}
