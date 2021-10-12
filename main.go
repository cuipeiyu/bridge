package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/atomic"
)

// worked like `rinetd`.

var wg sync.WaitGroup
var ctx, ctxCancel = context.WithCancel(context.Background())
var statInterval = time.Minute
var isNotExit = false
var tcpCounter atomic.Int64
var udpCounter atomic.Int64
var udpSsnMap = sync.Map{}
var udpSsnTTL = time.Minute
var chains []*chain

// forward TCP/UDP from ListenAddr to ToAddr
type chain struct {
	ListenAddr string
	Proto      string
	ToAddr     string
}

func (c *chain) String() string {
	return fmt.Sprintf("%v->%v/%v", c.ListenAddr, c.ToAddr, c.Proto)
}

// 可以通过 WaitCtx.Done 关闭 Closer
// 可以通过 返回值 chan 关闭 Closer
func registerCloseCnn0(c io.Closer) chan bool {
	cc := make(chan bool, 1)

	wg.Add(1)
	go func() {
		select {
		case <-ctx.Done():
		case <-cc:
		}
		_ = c.Close()
		wg.Done()
	}()
	return cc
}

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

/**
放弃的一种配置文件格式
rinetd.toml sample
[[Chans]]
ListenAddr="0.0.0.0:5678"
Proto="tcp"
PeerAddr="127.0.0.1:8100"
[[Chans]]
ListenAddr="0.0.0.0:5679"
Proto="tcp"
PeerAddr="127.0.0.1:8200"
parser sample
0.0.0.0 5678/tcp 127.0.0.1 8100/tcp
用上面的都太复杂了
*/

func setupChains() {
	// setupSignal()
	if len(chains) == 0 {
		// Logger.Info("no chains to work")
		ctxCancel()
		return
	}
	for _, c := range chains {
		wg.Add(1)
		go func(arg1 *chain) {
			if arg1.Proto == "tcp" {
				setupTCPChain(arg1)
			} else if arg1.Proto == "udp" {
				setupUDPChain(arg1)
			}
			wg.Done()
		}(c)
	}
}

func stat() {
	tc := time.Tick(statInterval)
	// logger := Logger.WithName("stat")
loop:
	for isNotExit {
		select {
		case <-ctx.Done():
			break loop
		case <-tc:
			log.Printf("[I] stat tcp:%3d, udp:%3d", tcpCounter.Load(), udpCounter.Load())
		}
	}
}

func main() {
	log.Printf("[I] 启动中。。。")

	loadConfigFile()

	setupChains()
	stat()

	// 信号监听
	state := 1
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

EXIT:
	for {
		sig := <-sc
		log.Printf("[I] 收到退出信号: %s", sig.String())
		switch sig {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			state = 0
			break EXIT
		case syscall.SIGHUP:
		default:
			break EXIT
		}
	}

	ctxCancel()
	isNotExit = true

	// Logger.Info("all work exit")
	log.Print("[I] 等待退出")
	wg.Wait()

	log.Print("[I] 结束")
	os.Exit(state)
}

func loadConfigFile() {
	log.Printf("[D] 读取配置文件")

	dir, _ := os.Getwd()
	filename := filepath.Join(dir, "bridge.conf")
	log.Printf("[D] 配置文件: %s", filename)

	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		log.Fatalf("[F] 配置文件不存在")
		return
	}

	fr, err := os.Open(filename)
	if err != nil {
		// Logger.Error(err, "error openfile", "filename", filename)
		return
	}
	defer fr.Close()

	// 逐行扫描
	sc := bufio.NewScanner(fr)
	for sc.Scan() {
		t := sc.Text()
		t = strings.TrimSpace(t)
		if len(t) <= 0 || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") {
			continue
		}

		ar := strings.Fields(t)
		if len(ar) < 3 {
			continue
		}
		arValid := make([]string, 0)
		for _, e := range ar {
			e = strings.TrimSpace(e)
			if len(e) > 0 {
				arValid = append(arValid, e)
			}
		}
		if len(arValid) > 2 {
			v := &chain{}
			v.ListenAddr = arValid[0]
			v.ToAddr = arValid[1]
			v.Proto = strings.ToLower(arValid[2])
			log.Printf("[D] 识别内容 %s", v.String())
			chains = append(chains, v)
		}
	}
}
