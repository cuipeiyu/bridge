package main

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func newUdpSsn(ctx context.Context, c *chain, fromAddr net.Addr, fromCnn net.PacketConn) *udpSession {
	var d net.Dialer
	toCnn, err := d.DialContext(ctx, c.Proto, c.ToAddr)
	if err != nil {
		log.Printf("[E] error dial ToAddr: %v", err)
		return nil
	}
	u := &udpSession{}
	u.WriteTime.Store(time.Now())
	u.ToCnn = toCnn
	u.FromAddr = fromAddr
	u.FromCnn = fromCnn
	u.OwnerChain = c
	return u
}

// Keep udp session
type udpSession struct {
	FromCnn    net.PacketConn
	FromAddr   net.Addr
	OwnerChain *chain
	WriteTime  atomic.Value
	ToCnn      net.Conn
}

func (u *udpSession) Close() error {
	return u.ToCnn.Close()
}

// forwardUDP 只转发 ToCnn -> FromAddr 方向的 UDP 报文。
// SetReadDeadline 协助完成老化功能。
func forwardUDP(us *udpSession) {
	b := make([]byte, 64*1024)
	for {
		// add one more second to make sure sub time > ttl
		t := time.Now().Add(udpSsnTTL).Add(time.Second)
		_ = us.ToCnn.SetReadDeadline(t)
		n, err := us.ToCnn.Read(b)
		_ = us.ToCnn.SetReadDeadline(time.Time{})
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if udpSsnCanAge(us, udpSsnTTL) {
					break
				}
			} else if errors.Is(err, syscall.ECONNREFUSED) {
				// If peer not UDP listen, then we will recv ICMP
				// golang will give error net.OpError ECONNREFUSED
				// we ignore this error
				continue
			} else {
				break
			}
		} else {
			_, err = us.FromCnn.WriteTo(b[:n], us.FromAddr)
			if err != nil {
				break
			}
		}
	}
}

func udpSsnCanAge(us *udpSession, ttl time.Duration) bool {
	now := time.Now()
	whenWriteVoid := us.WriteTime.Load()
	if whenWriteVoid == nil {
		return true
	}
	whenWrite := whenWriteVoid.(time.Time)
	return now.After(whenWrite) && now.Sub(whenWrite) > ttl
}

func setupUDPChain(ctx context.Context, c *chain) {
	defer wg.Done()

	var lc net.ListenConfig
	if ctl := v6OnlyControl(c.ListenAddr); ctl != nil {
		lc.Control = ctl
	}
	pktCnn, err := lc.ListenPacket(ctx, c.Proto, c.ListenAddr)
	if err != nil {
		log.Printf("[E] udp listen error: [%s][%s] %v", c.Proto, c.ListenAddr, err)
		return
	}
	serveUDP(ctx, pktCnn, c)
}

func serveUDP(ctx context.Context, pktCnn net.PacketConn, c *chain) {
	defer pktCnn.Close()

	OnInterrupt(func() {
		_ = pktCnn.Close()
	})

	// 每条链独立的会话表，key 为客户端地址，避免多条链之间会话互窜
	var ssnMap sync.Map
	rbuf := make([]byte, 128*1024)
	for {
		rsize, raddr, err := pktCnn.ReadFrom(rbuf)
		if err != nil {
			break
		}

		ssnKey := raddr.String()
		var ssn *udpSession
		if v, ok := ssnMap.Load(ssnKey); ok {
			ssn = v.(*udpSession)
		} else {
			// 仅在会话不存在时才 dial，避免每个包都创建并丢弃 socket 导致 fd 泄漏
			ssn = newUdpSsn(ctx, c, raddr, pktCnn)
			if ssn == nil {
				continue
			}
			udpCounter.Add(1)
			ssnMap.Store(ssnKey, ssn)
			log.Printf("[I] udp session %s via %s", ssnKey, c.String())
			wg.Add(1)
			go func(s *udpSession) {
				defer wg.Done()
				defer func() {
					udpCounter.Add(-1)
					ssnMap.Delete(ssnKey)
				}()

				// 监听退出信号及时关闭会话，否则进程退出时 Read 会阻塞到 TTL 超时
				watchDone := make(chan struct{})
				defer close(watchDone)
				go func() {
					select {
					case <-ctx.Done():
						_ = s.Close()
					case <-watchDone:
					}
				}()
				forwardUDP(s)
				_ = s.Close()
			}(ssn)
		}

		_, err = ssn.ToCnn.Write(rbuf[:rsize])
		if err != nil {
			_ = ssn.Close()
			ssnMap.Delete(ssnKey)
			continue
		}
		ssn.WriteTime.Store(time.Now())
	}
}
