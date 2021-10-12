package main

import (
	"io"
	"net"
	"sync"
)

// 有的人是双向中任何一个方向断开，都会断开双向
// 代码这样写, 技巧：还使用到了 sync.Once 只运行 1 次。
// var once sync.Once
// go func() {
// 	io.Copy(connection, bashf)
// 	once.Do(close)
// }()
// go func() {
// 	io.Copy(bashf, connection)
// 	once.Do(close)
// }()
func forwardTCP(c *chain, left io.ReadWriteCloser, right io.ReadWriteCloser) {
	_ = c
	wg := new(sync.WaitGroup)
	bothClose := make(chan bool, 1)
	// right -> left
	wg.Add(1)
	go func() {
		b := make([]byte, 512*1024)
		_, _ = io.CopyBuffer(left, right, b)
		wg.Done()
	}()

	// left -> right
	wg.Add(1)
	go func() {
		b := make([]byte, 512*1024)
		_, _ = io.CopyBuffer(right, left, b)
		wg.Done()
	}()

	// wait read & write close
	wg.Add(1)
	go func() {
		wg.Wait()
		close(bothClose)
		wg.Done()
	}()

	select {
	case <-bothClose:
	case <-ctx.Done():
	}
	_ = left.Close()
	_ = right.Close()
}

func setupTCPChain(c *chain) {
	var err error
	// logger := Logger.WithName("setupTCPChain")
	// logger = logger.WithValues("chain", c.String())
	// logger.Info("enter")
	// defer logger.Info("leave")

	var lc net.ListenConfig
	sn, err := lc.Listen(ctx, c.Proto, c.ListenAddr)
	if err != nil {
		// logger.Error(err, "error listen")
		return
	}
	closeChan := registerCloseCnn0(sn)
	for {
		cnn, err := sn.Accept()
		if err != nil {
			// logger.Error(err, "error accept")
			break
		}

		// connect peer
		d := new(net.Dialer)
		toCnn, err := d.DialContext(ctx, c.Proto, c.ToAddr)
		if err != nil {
			// logger.Error(err, "error dial ToAddr")
			continue
		}
		// logger.Info("new connection pair",
		// "1From", cnn.RemoteAddr().String(), "1To", cnn.LocalAddr().String(),
		// "2From", toCnn.LocalAddr().String(), "2To", toCnn.RemoteAddr().String())
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
