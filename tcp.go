package main

import (
	"io"
	"log"
	"net"
	"sync"
	"time"
)

func forwardTCP(c *chain, from io.ReadWriteCloser, to io.ReadWriteCloser) {
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

	log.Printf("[I] 连接关闭")
	_ = from.Close()
	_ = to.Close()
}

func setupTCPChain(c *chain) {
	var err error

	var lc net.ListenConfig
	sn, err := lc.Listen(ctx, c.Proto, c.ListenAddr)
	if err != nil {
		// logger.Error(err, "error listen")
		return
	}
	closeChan := registerCloseCnn0(sn)
	for {
		// 连接到目标
		d := new(net.Dialer)
		toCnn, err := d.DialContext(ctx, c.Proto, c.ToAddr)
		if err != nil {
			// logger.Error(err, "error dial ToAddr")
			continue
		}

		// 新连接
		cnn, err := sn.Accept()
		if err != nil {
			// logger.Error(err, "error accept")
			break
		}

		log.Printf("[I] 已初始化新连接")

		// 设置读写超时
		cnn.SetDeadline(time.Now().Add(time.Minute))

		// 协程中
		// 处理收发
		wg.Add(1)
		go func(arg1 *chain, arg2 io.ReadWriteCloser, arg3 io.ReadWriteCloser) {
			defer wg.Done()

			tcpCounter.Add(1)
			forwardTCP(arg1, arg2, arg3)
			tcpCounter.Sub(1)
		}(c, cnn, toCnn)

	}
	close(closeChan)
}
