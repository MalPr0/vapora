//go:build unix

package main

import (
	"os"
	"syscall"
	"unsafe"
)

// Raw mode, so a key moves the paddle instead of waiting for enter.
//
// It is here rather than imported because the point of this example is that
// pkg/ is all you need for the network. The terminal is your own problem, and
// this is what it costs: about thirty lines.
type terminal struct {
	fd    int
	saved syscall.Termios
}

func rawMode() (*terminal, error) {
	fd := int(os.Stdin.Fd())

	var saved syscall.Termios
	if err := ioctl(fd, tcGets, &saved); err != nil {
		return nil, err
	}

	raw := saved
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	raw.Iflag &^= syscall.IXON | syscall.ICRNL
	// One byte at a time, no waiting: a game reads whatever is there and gets
	// on with the frame.
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	if err := ioctl(fd, tcSets, &raw); err != nil {
		return nil, err
	}
	return &terminal{fd: fd, saved: saved}, nil
}

func (t *terminal) restore() {
	_ = ioctl(t.fd, tcSets, &t.saved)
}

func ioctl(fd int, request uintptr, termios *syscall.Termios) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), request,
		uintptr(unsafe.Pointer(termios)), 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

// keys reads the keyboard onto a channel, so the game loop can select on it
// instead of blocking. A read that nothing can interrupt would otherwise mean
// the game only reacts when somebody presses something.
func keys() <-chan byte {
	pressed := make(chan byte, 16)

	go func() {
		defer close(pressed)

		buffer := make([]byte, 8)
		for {
			read, err := os.Stdin.Read(buffer)
			if err != nil {
				return
			}
			for _, key := range buffer[:read] {
				select {
				case pressed <- key:
				default: // a full buffer means the player is ahead of the game
				}
			}
		}
	}()

	return pressed
}
