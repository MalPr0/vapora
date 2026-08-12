//go:build darwin

package main

import "syscall"

const (
	tcGets = syscall.TIOCGETA
	tcSets = syscall.TIOCSETA
)
