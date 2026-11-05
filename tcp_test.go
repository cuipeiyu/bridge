package main

import (
	"io"
	"net"
	"testing"
	"time"
)

func startTCPUpstream(t *testing.T) net.Addr {
	t.Helper()
	up, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start upstream: %v", err)
	}
	t.Cleanup(func() { _ = up.Close() })
	go func() {
		for {
			c, err := up.Accept()
			if err != nil {
				return
			}
			go func(cc net.Conn) {
				defer cc.Close()
				_, _ = io.Copy(cc, cc) // echo
			}(c)
		}
	}()
	return up.Addr()
}

func TestTCPForward(t *testing.T) {
	upAddr := startTCPUpstream(t)
	listenAddr := startTCPChain(t, &chain{ToAddr: upAddr.String(), Proto: "tcp"})

	conn, err := net.Dial("tcp", listenAddr.String())
	if err != nil {
		t.Fatalf("dial bridge: %v", err)
	}
	defer conn.Close()

	msg := []byte("hello bridge")
	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := make([]byte, len(msg))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(got) != string(msg) {
		t.Fatalf("echo mismatch: got %q, want %q", got, msg)
	}
}

func TestTCPDialFailureClosesClientConn(t *testing.T) {
	// 上游端口不可达，客户端连接应被立即关闭而不是挂起
	listenAddr := startTCPChain(t, &chain{ToAddr: "127.0.0.1:1", Proto: "tcp"})

	conn, err := net.Dial("tcp", listenAddr.String())
	if err != nil {
		t.Fatalf("dial bridge: %v", err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 16)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("expected conn to be closed, but read succeeded")
	}
}

func TestTCPForwardClosesWhenClientCloses(t *testing.T) {
	// 客户端关闭后，双向连接应被释放，转发协程不应永久挂起
	upAddr := startTCPUpstream(t)
	listenAddr := startTCPChain(t, &chain{ToAddr: upAddr.String(), Proto: "tcp"})

	conn, err := net.Dial("tcp", listenAddr.String())
	if err != nil {
		t.Fatalf("dial bridge: %v", err)
	}

	msg := []byte("ping")
	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := make([]byte, len(msg))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read echo: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		// 客户端主动关闭，forwardTCP 应随之退出
		_ = conn.Close()
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("client close did not complete in time")
	}

	before := tcpCounter.Load()
	// 等待桥接侧释放计数
	deadline := time.Now().Add(3 * time.Second)
	for tcpCounter.Load() >= before && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if tcpCounter.Load() >= before {
		t.Fatal("bridge did not release connection after client close")
	}
}
