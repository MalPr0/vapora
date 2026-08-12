//go:build !darwin && !linux

package tui

import (
	"errors"
	"os"
)

// Terminal is unavailable outside the unix family here, because raw mode needs
// platform specific syscalls and the project carries no dependencies. Callers
// fall back to line mode, which works everywhere.
type Terminal struct{}

func IsTerminal(*os.File) bool { return false }

func OpenTerminal(*os.File, *os.File) (*Terminal, error) {
	return nil, errors.New("tui: raw mode is not supported on this platform")
}

func (t *Terminal) Restore()                  {}
func (t *Terminal) Size() (int, int)          { return 80, 24 }
func (t *Terminal) Resized() <-chan os.Signal { return nil }
func (t *Terminal) In() *os.File              { return nil }
func (t *Terminal) Out() *os.File             { return nil }
