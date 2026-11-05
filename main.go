package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// worked like `rinetd`.

var wg sync.WaitGroup
var ctx, ctxCancel = context.WithCancel(context.Background())
var tcpCounter atomic.Int64
var udpCounter atomic.Int64
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

func setupChains() {
	if len(chains) == 0 {
		ctxCancel()
		return
	}

	for _, c := range chains {
		if c.Proto == "tcp" {
			wg.Add(1)
			go setupTCPChain(ctx, c)
		} else if c.Proto == "udp" {
			wg.Add(1)
			go setupUDPChain(ctx, c)
		}
	}
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
}

func loadConfigFile() {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("[F] get working directory failed: %v", err)
	}
	filename := filepath.Join(dir, "bridge.conf")
	log.Printf("[D] ready to load config file: %s", filename)

	fr, err := os.Open(filename)
	if err != nil {
		log.Fatalf("[F] open the config file failed: %v", err)
	}
	defer fr.Close()

	chains = append(chains, parseConfig(fr)...)
}

// parseConfig 从 r 中逐行解析转发规则，格式：ListenAddr ToAddr Proto
// 跳过空行、# 和 // 注释，Proto 仅接受 tcp/udp（大小写不敏感），非法行记录日志后跳过。
func parseConfig(r io.Reader) []*chain {
	var cs []*chain
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		t := strings.TrimSpace(sc.Text())
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") {
			continue
		}

		f := strings.Fields(t)
		if len(f) < 3 {
			log.Printf("[E] invalid config line, skipped: %s", t)
			continue
		}

		proto := strings.ToLower(f[2])
		if proto != "tcp" && proto != "udp" {
			log.Printf("[E] invalid proto %q, skipped: %s", f[2], t)
			continue
		}

		v := &chain{
			ListenAddr: f[0],
			ToAddr:     f[1],
			Proto:      proto,
		}
		log.Printf("[D] got config content: %s", v.String())
		cs = append(cs, v)
	}
	return cs
}
