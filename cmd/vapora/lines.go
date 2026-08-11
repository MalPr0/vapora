package main

import (
	"bufio"
	"context"
	"os"
)

// readLines turns the terminal into a channel.
//
// bufio.Scanner blocks in a read that nothing can interrupt: not a cancelled
// context, not a signal. A plain mode session that waits on it directly ignores
// ctrl+c until the next time somebody presses enter, which reads as a hung
// program. Reading here instead means the session can watch its context and
// leave; the goroutine stays parked in that read until the process exits, which
// is what a process exiting is for.
func readLines(ctx context.Context) <-chan string {
	lines := make(chan string)

	go func() {
		defer close(lines)

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
	}()

	return lines
}
