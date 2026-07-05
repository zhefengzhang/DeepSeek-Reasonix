//go:build windows

package main

import "syscall"

func hideConsoleAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}
