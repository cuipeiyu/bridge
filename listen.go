package main

import (
	"net"
	"syscall"
)

// v6OnlyControl 针对纯 IPv6 监听地址（含通配 [::]）返回一个 Control，
// 将 socket 设为 IPV6_V6ONLY=1，使其不再接收 IPv4（v4-mapped）流量，
// 从而允许 0.0.0.0:P 与 [::]:P 在同进程内共存、分别指向不同上游。
//
// 仅当 ListenAddr 的 host 是纯 IPv6 字面量时生效：
//   - IPv4（0.0.0.0、1.2.3.4）		-> 返回 nil，保持默认
//   - IPv4-mapped（::ffff:1.2.3.4）	-> 返回 nil，设了 V6ONLY 会 bind EINVAL
//   - hostname（example.com:80）		-> 返回 nil，交给解析结果
func v6OnlyControl(listenAddr string) func(network, address string, conn syscall.RawConn) error {
	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() != nil {
		return nil
	}
	return func(network, address string, conn syscall.RawConn) error {
		var serr error
		conn.Control(func(fd uintptr) {
			serr = setV6Only(fd)
		})
		return serr
	}
}
