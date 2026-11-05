package main

import (
	"net"
	"testing"
	"time"
)

// startUDPUpstream 启动一个带前缀应答的 UDP 上游：回复 = prefix + 原报文
func startUDPUpstream(t *testing.T, prefix byte) net.Addr {
	t.Helper()
	up, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start upstream: %v", err)
	}
	t.Cleanup(func() { _ = up.Close() })
	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := up.ReadFrom(buf)
			if err != nil {
				return
			}
			reply := append([]byte{prefix}, buf[:n]...)
			_, _ = up.WriteTo(reply, addr)
		}
	}()
	return up.LocalAddr()
}

func udpRoundTrip(t *testing.T, client net.PacketConn, chainAddr net.Addr, msg string, wantPrefix byte) {
	t.Helper()
	_ = client.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := client.WriteTo([]byte(msg), chainAddr); err != nil {
		t.Fatalf("write to bridge: %v", err)
	}
	buf := make([]byte, 2048)
	n, _, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if n != len(msg)+1 || buf[0] != wantPrefix || string(buf[1:n]) != msg {
		t.Fatalf("echo mismatch: got %q, want %q + %q", buf[:n], string(rune(wantPrefix)), msg)
	}
}

func TestUDPForward(t *testing.T) {
	upAddr := startUDPUpstream(t, 'A')
	listenAddr := startUDPChain(t, &chain{ToAddr: upAddr.String(), Proto: "udp"})

	client, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer client.Close()

	// 第一个包：建立会话
	udpRoundTrip(t, client, listenAddr, "first", 'A')
	// 第二个包：复用已有会话（不新建 socket）
	udpRoundTrip(t, client, listenAddr, "second", 'A')
}

func TestUDPChainsIsolation(t *testing.T) {
	// 两条链，同一个客户端源地址分别发包到两条链。
	// 修复前：全局会话表 key 只有客户端地址，第二条链会复用第一条链的会话，
	// 报文被发往错误的上游（回包前缀会是 'A' 而不是 'B'）。
	up1 := startUDPUpstream(t, 'A')
	up2 := startUDPUpstream(t, 'B')
	listen1 := startUDPChain(t, &chain{ToAddr: up1.String(), Proto: "udp"})
	listen2 := startUDPChain(t, &chain{ToAddr: up2.String(), Proto: "udp"})

	client, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer client.Close()

	udpRoundTrip(t, client, listen1, "to-chain-1", 'A')
	udpRoundTrip(t, client, listen2, "to-chain-2", 'B')
	udpRoundTrip(t, client, listen1, "again-chain-1", 'A')
	udpRoundTrip(t, client, listen2, "again-chain-2", 'B')
}
