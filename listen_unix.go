//go:build !windows

package main

import "syscall"

// setV6Only Unix 实现：fd 是 int
func setV6Only(fd uintptr) error {
	return syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, syscall.IPV6_V6ONLY, 1)
}
