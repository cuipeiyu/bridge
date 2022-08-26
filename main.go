package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
// func registerCloseCnn0(c io.Closer) chan bool {
// 	cc := make(chan bool, 1)

// 	wg.Add(1)
// 	go func() {
// 		select {
// 		case <-ctx.Done():
// 		case <-cc:
// 		}
// 		_ = c.Close()
// 		wg.Done()
// 	}()
// 	return cc
// }

func setupChains() {
	if len(chains) == 0 {
		ctxCancel()
		return
	}

	for _, c := range chains {
		if c.Proto == "tcp" {
			wg.Add(1)
			go setupTCPChain(c)
		} else if c.Proto == "udp" {
			wg.Add(1)
			go setupUDPChain(c)
		}
	}
}

func showStat() {
	log.Printf("[I] stat tcp:%3d, udp:%3d", tcpCounter.Load(), udpCounter.Load())
}

func main() {
	log.Printf("[I] starting...")

	loadConfigFile()

	setupChains()

	OnInterrupt(func() {
		log.Printf("[I] begin exit...")

		ctxCancel()
	})

	wg.Wait()

	log.Print("[I] wait all down...")
	Wait()

	log.Print("[I] exit success")
	os.Exit(0)
}

func loadConfigFile() {
	log.Printf("[D] begin load config file")

	dir, _ := os.Getwd()
	filename := filepath.Join(dir, "bridge.conf")
	log.Printf("[D] ready to load config file: %s", filename)

	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		log.Fatalf("[F] config file not exists")
		return
	}

	fr, err := os.Open(filename)
	if err != nil {
		log.Fatalf("[F] open the config file failed %v", err)
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
			log.Printf("[D] got config content: %s", v.String())
			chains = append(chains, v)
		}
	}
}
