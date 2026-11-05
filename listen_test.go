package main

import "testing"

// v6OnlyControl 只对纯 IPv6 字面量返回 Control，其余情况必须返回 nil。
func TestV6OnlyControl(t *testing.T) {
	cases := []struct {
		listenAddr string
		wantCtl    bool
	}{
		{"[::]:80", true},            // v6 通配，核心场景
		{"[2001:db8::]:443", true},   // 具体 v6 地址
		{"[::1]:6379", true},         // v6 loopback
		{"0.0.0.0:80", false},        // v4 通配
		{"127.0.0.1:99", false},      // v4 loopback
		{"[::ffff:1.2.3.4]:80", false}, // v4-mapped，设 V6ONLY 会 bind EINVAL
		{"example.com:80", false},    // hostname，交给解析结果
		{"no-port", false},           // 非法格式，不处理
		{"", false},
	}
	for _, c := range cases {
		if got := v6OnlyControl(c.listenAddr) != nil; got != c.wantCtl {
			t.Errorf("v6OnlyControl(%q) ctl=%v, want %v", c.listenAddr, got, c.wantCtl)
		}
	}
}
