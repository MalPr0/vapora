package tcpchat

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/MalPr0/vapora/pkg/punch"
	"github.com/MalPr0/vapora/pkg/text"
)

const dialTimeout = 10 * time.Second

// Dial joins a hosted chat. The secret comes from the invite, so a wrong one
// fails at the first frame rather than letting a stranger talk.
func Dial(ctx context.Context, address string, secret punch.Secret, input io.Reader, output io.Writer) error {
	codec, err := punch.NewSecretCodec(secret, punch.RoleJoiner)
	if err != nil {
		return err
	}

	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("chat: cannot connect to %s: %w", address, err)
	}
	defer conn.Close()

	peer := newStream(conn, codec)

	var quitting atomic.Bool
	closeSession := func() {
		quitting.Store(true)
		_ = peer.Close()
	}

	go func() {
		<-ctx.Done()
		closeSession()
	}()

	// Closing stdin ends the session, which keeps the client usable both
	// interactively (ctrl+d) and from a pipe.
	go func() {
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			if err := peer.WriteLine(scanner.Text()); err != nil {
				break
			}
		}
		closeSession()
	}()

	for {
		line, err := peer.ReadLine()
		if err != nil {
			if quitting.Load() || err == io.EOF || ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("chat: connection to %s dropped: %w", address, err)
		}
		fmt.Fprintln(output, text.Safe(line))
	}
}
