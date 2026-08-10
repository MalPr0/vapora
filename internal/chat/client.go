package chat

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/MalPr0/vapora/pkg/text"
)

const dialTimeout = 10 * time.Second

// Dial connects to a chat server and pumps stdin and the socket into each other
// until either side closes.
func Dial(ctx context.Context, address string, input io.Reader, output io.Writer) error {
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("chat: cannot connect to %s: %w", address, err)
	}
	defer conn.Close()

	var quitting atomic.Bool
	closeSession := func() {
		quitting.Store(true)
		_ = conn.Close()
	}

	go func() {
		<-ctx.Done()
		closeSession()
	}()

	// Closing stdin ends the session, which keeps the client usable both
	// interactively (ctrl+d) and from a pipe.
	go func() {
		Pump(input, conn)
		closeSession()
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		fmt.Fprintln(output, text.Safe(scanner.Text()))
	}
	if err := scanner.Err(); err != nil && !quitting.Load() {
		return fmt.Errorf("chat: connection to %s dropped: %w", address, err)
	}
	return nil
}

// Pump forwards every line read from input into the writer.
func Pump(input io.Reader, target io.Writer) {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		fmt.Fprintf(target, "%s\n", scanner.Text())
	}
}
