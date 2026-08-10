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

func (i Invite) Token() string {
	if len(i.Secret) == 0 {
		return i.Endpoint.String()
	}
	return i.Endpoint.String() + secretSeparator + i.Secret.String()
}

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

	token := fields[len(fields)-1]
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
