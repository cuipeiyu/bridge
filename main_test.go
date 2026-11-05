package main

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	input := `# 注释
   // 注释
0.0.0.0:80 127.0.0.1:99 tcp
1.2.3.4:53 5.6.7.8:53 UDP
0.0.0.0:81 127.0.0.1:100 sctp
only-two-fields
`
	cs := parseConfig(strings.NewReader(input))
	if len(cs) != 2 {
		t.Fatalf("expected 2 chains, got %d: %+v", len(cs), cs)
	}

	want := []chain{
		{ListenAddr: "0.0.0.0:80", ToAddr: "127.0.0.1:99", Proto: "tcp"},
		{ListenAddr: "1.2.3.4:53", ToAddr: "5.6.7.8:53", Proto: "udp"},
	}
	for i, w := range want {
		if *cs[i] != w {
			t.Errorf("chain[%d] = %+v, want %+v", i, *cs[i], w)
		}
	}
}

// startTCPChain 测试自绑端口后注入 serveTCP，返回实际监听地址，避免 free-port 竞态
func startTCPChain(t *testing.T, c *chain) net.Addr {
	t.Helper()
	sn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = sn.Close() })
	c.ListenAddr = sn.Addr().String()
	testCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go serveTCP(testCtx, sn, c)
	return sn.Addr()
}

// startUDPChain 测试自绑端口后注入 serveUDP，返回实际监听地址
func startUDPChain(t *testing.T, c *chain) net.Addr {
	t.Helper()
	p, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen packet: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	c.ListenAddr = p.LocalAddr().String()
	testCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go serveUDP(testCtx, p, c)
	return p.LocalAddr()
}
