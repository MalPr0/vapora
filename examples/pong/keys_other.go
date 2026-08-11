//go:build !unix

package main

import "errors"

// Windows needs its console API rather than termios. The network half of this
// example is portable; the terminal half is not, and pretending otherwise would
// build and then misbehave.
type terminal struct{}

func rawMode() (*terminal, error) {
	return nil, errors.New("this example needs a unix terminal; the pkg/ side is portable")
}

func (t *terminal) restore() {}

func keys() <-chan byte {
	closed := make(chan byte)
	close(closed)
	return closed
}
