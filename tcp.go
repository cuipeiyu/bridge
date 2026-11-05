package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
)

// forwardTCP 双向转发。任一方向结束（EOF/错误）即双向关闭，
// 中断另一个方向，避免半开连接长期挂起泄漏。
func forwardTCP(from io.ReadWriteCloser, to io.ReadWriteCloser) {
	done := make(chan struct{}, 2)

	copyFn := func(dst io.WriteCloser, src io.ReadCloser) {
		_, _ = io.Copy(dst, src)
		done <- struct{}{}
	}

	go copyFn(to, from)
	go copyFn(from, to)

	<-done
	_ = from.Close()
	_ = to.Close()
	<-done

	log.Printf("[I] release tcp connection")
}

func setupTCPChain(ctx context.Context, c *chain) {
	defer wg.Done()

	var lc net.ListenConfig
	sn, err := lc.Listen(ctx, c.Proto, c.ListenAddr)
	if err != nil {
		log.Printf("[E] new listen error: [%s][%s] %v", c.Proto, c.ListenAddr, err)
		return
	}
	serveTCP(ctx, sn, c)
}

func serveTCP(ctx context.Context, sn net.Listener, c *chain) {
	defer sn.Close()

	OnInterrupt(func() {
		_ = sn.Close()
	})

	for {
		cnn, err := sn.Accept()
		if errors.Is(err, net.ErrClosed) {
			break
		}
		if err != nil {
			log.Printf("[E] accept error: %v", err)
			continue
		}

		wg.Add(1)
		go func(arg1 *chain, arg2 net.Conn) {
			defer wg.Done()

			var d net.Dialer
			to, err := d.DialContext(ctx, arg1.Proto, arg1.ToAddr)
			if err != nil {
				_ = arg2.Close()
				log.Printf("[E] dial ToAddr error: %v", err)
				return
			}

			tcpCounter.Add(1)
			log.Printf("[I] tcp conn %s via %s", arg2.RemoteAddr(), arg1.String())

			forwardTCP(arg2, to)
			tcpCounter.Add(-1)
		}(c, cnn)
	}
}
