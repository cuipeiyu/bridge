package main

import (
	"errors"
	"io"
	"log"
	"net"
	"sync"
)

func forwardTCP(from io.ReadWriteCloser, to io.ReadWriteCloser) {
	g := new(sync.WaitGroup)
	bothClose := make(chan bool, 1)

	// to -> from
	g.Add(1)
	go func() {
		defer g.Done()
		if _, err := io.Copy(from, to); err != nil {
			return
		}
	}()

	// from -> to
	g.Add(1)
	go func() {
		defer g.Done()
		if _, err := io.Copy(to, from); err != nil {
			return
		}
	}()

	go func() {
		g.Wait()
		close(bothClose)
	}()

	select {
	case <-bothClose: // 等待关闭
	case <-ctx.Done(): // 等待退出
	}

	log.Printf("[I] release tcp connection")
	_ = from.Close()
	_ = to.Close()
}

func setupTCPChain(c *chain) {
	defer wg.Done()

	var err error

	var lc net.ListenConfig
	sn, err := lc.Listen(ctx, c.Proto, c.ListenAddr)
	if err != nil {
		log.Printf("[E] new listen error: [%s][%s] %v", c.Proto, c.ListenAddr, err)
		return
	}
	defer sn.Close()

	OnInterrupt(func() {
		sn.Close()
	})

	for {
		// 新连接
		cnn, err := sn.Accept()
		if errors.Is(err, net.ErrClosed) {
			break
		}
		if err != nil {
			log.Printf("[E] accept error: %v", err)
			continue
		}

		// 协程中
		// 处理收发
		wg.Add(1)
		go func(arg1 *chain, arg2 net.Conn) {
			defer wg.Done()

			// 连接到目标
			d := new(net.Dialer)
			to, err := d.DialContext(ctx, arg1.Proto, arg1.ToAddr)
			if err != nil {
				// 目标无法连接，不建立通信，并关闭连接
				arg2.Close()

				log.Printf("[E] dial ToAddr error: %v", err)
				return
			}

			// 设置读写超时
			// arg2.SetDeadline(time.Now().Add(time.Minute))

			tcpCounter.Add(1)

			// 打印统计
			showStat()

			forwardTCP(arg2, to)
			tcpCounter.Sub(1)

			// 打印统计
			showStat()
		}(c, cnn)
	}
}
