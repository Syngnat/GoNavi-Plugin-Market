//go:build windows

package main

import "syscall"

// chromeSysProcAttr keeps the login browser from flashing a console window.
func chromeSysProcAttr() *syscall.SysProcAttr {
	const createNoWindow = 0x08000000
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
