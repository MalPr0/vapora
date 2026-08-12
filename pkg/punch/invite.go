package punch

import (
	"fmt"
	"net"
	"strings"
)

const secretSeparator = "/"

// Invite is the line a peer copies and runs to join. Keeping it a runnable
// command is the whole point: the receiver pastes it into a shell as is. The
// secret rides along as part of the endpoint token so the whole thing stays a
// single word, safe to paste after the command.
type Invite struct {
	Endpoint *net.UDPAddr
	Secret   Secret
}

// Token is the endpoint and secret without the command around them.
func (i Invite) Token() string {
	if len(i.Secret) == 0 {
		return i.Endpoint.String()
	}
	return i.Endpoint.String() + secretSeparator + i.Secret.String()
}

// Command renders the invite as a line the other side can paste and run,
// which is the whole of the user interface for connecting.
func (i Invite) Command(command string) string {
	return fmt.Sprintf("%s %s", command, i.Token())
}

// ParseInvite accepts a bare endpoint, an endpoint with a secret, or a whole
// pasted command, so the user never has to trim anything by hand.
func ParseInvite(line string) (Invite, error) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return Invite{}, fmt.Errorf("punch: %q carries no endpoint", line)
	}

	token := joinToken(fields)
	endpointPart, secretPart, hasSecret := strings.Cut(token, secretSeparator)

	endpoint, err := net.ResolveUDPAddr("udp4", endpointPart)
	if err != nil {
		return Invite{}, fmt.Errorf("punch: %q is not a host:port endpoint: %w", endpointPart, err)
	}
	if endpoint.Port == 0 {
		return Invite{}, fmt.Errorf("punch: %q carries no port", endpointPart)
	}

	invite := Invite{Endpoint: endpoint}
	if hasSecret {
		secret, err := ParseSecret(secretPart)
		if err != nil {
			return Invite{}, err
		}
		invite.Secret = secret
	}
	return invite, nil
}

// joinToken puts back together a token a terminal wrapped. An invite is longer
// than 80 columns, so it reaches the other side split across lines as often as
// not, and a paste that decodes to half a key is the worst possible failure:
// it looks like a typo in a random string nobody can proofread.
//
// Nothing legitimate follows the token, so everything from the field carrying
// the separator onwards belongs to it.
func joinToken(fields []string) string {
	for i, field := range fields {
		// A path in the command that runs this also holds separators, so the
		// endpoint in front of it is what tells a token from `./cmd/vapora`.
		if endpoint, _, found := strings.Cut(field, secretSeparator); found &&
			strings.Contains(endpoint, ":") {
			return strings.Join(fields[i:], "")
		}
	}
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}
