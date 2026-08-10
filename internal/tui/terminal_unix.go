//go:build darwin || linux

package tui

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

type winsize struct {
	rows, columns, xpixel, ypixel uint16
}

// Terminal owns the tty: raw mode on the way in, and the original settings back
// on the way out no matter how the program ends.
type Terminal struct {
	in       *os.File
	out      *os.File
	original syscall.Termios
	restored bool
	resized  chan os.Signal
}

// IsTerminal reports whether the file is a tty, which is what decides between
// the full UI and the plain line mode a pipe needs.
func IsTerminal(file *os.File) bool {
	var state syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, file.Fd(),
		uintptr(ioctlReadTermios), uintptr(unsafe.Pointer(&state)), 0, 0, 0)
	return errno == 0
}

func OpenTerminal(in, out *os.File) (*Terminal, error) {
	terminal := &Terminal{in: in, out: out, resized: make(chan os.Signal, 1)}

	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, in.Fd(),
		uintptr(ioctlReadTermios), uintptr(unsafe.Pointer(&terminal.original)), 0, 0, 0); errno != 0 {
		return nil, fmt.Errorf("tui: cannot read the terminal state: %w", errno)
	}

	raw := terminal.original
	// Canonical mode buffers until Enter and echo would double every key, and
	// neither can stay on for a UI that renders its own input line.
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN
	raw.Iflag &^= syscall.ICRNL | syscall.IXON
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, in.Fd(),
		uintptr(ioctlWriteTermios), uintptr(unsafe.Pointer(&raw)), 0, 0, 0); errno != 0 {
		return nil, fmt.Errorf("tui: cannot enter raw mode: %w", errno)
	}

	signal.Notify(terminal.resized, syscall.SIGWINCH)
	// The alternate screen keeps the shell scrollback intact, and hiding the
	// cursor stops it from strobing across a redrawing frame.
	fmt.Fprint(out, "\x1b[?1049h\x1b[?25l\x1b[2J")
	return terminal, nil
}

// Restore puts the terminal back. It is safe to call twice, because both the
// deferred path and a signal handler will want to.
func (t *Terminal) Restore() {
	if t.restored {
		return
	}
	t.restored = true

	signal.Stop(t.resized)
	fmt.Fprint(t.out, "\x1b[0m\x1b[?25h\x1b[?1049l")
	syscall.Syscall6(syscall.SYS_IOCTL, t.in.Fd(),
		uintptr(ioctlWriteTermios), uintptr(unsafe.Pointer(&t.original)), 0, 0, 0)
}

func (t *Terminal) Size() (int, int) {
	var size winsize
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, t.out.Fd(),
		uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&size))); errno != 0 || size.columns == 0 {
		return 80, 24
	}
	return int(size.columns), int(size.rows)
}

func (t *Terminal) Resized() <-chan os.Signal { return t.resized }

func (t *Terminal) In() *os.File  { return t.in }
func (t *Terminal) Out() *os.File { return t.out }
