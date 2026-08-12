//go:build linux

package main

import "syscall"

const (
	tcGets = syscall.TCGETS
	tcSets = syscall.TCSETS
)
