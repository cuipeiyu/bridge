package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
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
	tc := time.NewTicker(10 * time.Second)
	defer tc.Stop()
	// logger := Logger.WithName("stat")
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-tc.C:
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
			// 跳过
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
