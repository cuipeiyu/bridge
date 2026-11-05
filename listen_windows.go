//go:build windows

package main

import "syscall"

// setV6Only Windows 实现：socket 是 Handle 类型，与 Unix 的 int 不同
func setV6Only(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_IPV6, syscall.IPV6_V6ONLY, 1)
}
