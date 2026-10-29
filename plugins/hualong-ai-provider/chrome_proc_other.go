//go:build !windows

package main

import "syscall"

// chromeSysProcAttr adds no process attributes outside Windows.
func chromeSysProcAttr() *syscall.SysProcAttr { return nil }
