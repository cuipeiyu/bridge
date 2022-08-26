package main

import (
	"errors"
	"log"
	"net"
	"sync/atomic"
	"syscall"
	"time"
)

func newUdpSsn(c *chain, fromAddr net.Addr,
	fromCnn net.PacketConn) *udpSession {
	d := new(net.Dialer)
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

// 只转发 right -> left 方向的 UDP 报文
// SetReadDeadline 协助完成老化功能
func forwardUDP(us *udpSession) {
	// logger := Logger.WithName("forwardUDP")
	// logger = logger.WithValues("chain", us.OwnerChain.String())
	// logger = logger.WithValues("fromAddr", us.FromAddr.String())
	// logger.Info("enter")
	b := make([]byte, 64*1024)
	// closeChan := registerCloseCnn0(us)
	for {
		// add one more second to make sure sub time > ttl
		t := time.Now().Add(udpSsnTTL).Add(time.Second)
		_ = us.ToCnn.SetReadDeadline(t)
		// this is a udp read
		n, err := us.ToCnn.Read(b)
		_ = us.ToCnn.SetReadDeadline(time.Time{})
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if udpSsnCanAge(us, udpSsnTTL) {
					// logger.Info("read timeout and aged")
					break
				}
			} else if errors.Is(err, syscall.ECONNREFUSED) {
				// If peer not UDP listen, then we will recv ICMP
				// golang will give error net.OpError ECONNREFUSED
				// we ignore this error
				continue
			} else {
				// logger.Error(err, "error read")
				break
			}
		} else {
			_, err = us.FromCnn.WriteTo(b[:n], us.FromAddr)
			if err != nil {
				// logger.Error(err, "error WriteTo")
				break
			}
		}
	}
	// close(closeChan)
	// logger.Info("leave")
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

func setupUDPChain(c *chain) {
	defer wg.Done()

	var err error
	// logger := Logger.WithName("setupUDPChain")
	// logger = logger.WithValues("chain", c.String())
	// logger.Info("enter")
	// defer logger.Info("leave")

	var lc net.ListenConfig
	pktCnn, err := lc.ListenPacket(ctx, c.Proto, c.ListenAddr)
	if err != nil {
		// logger.Error(err, "error ListenPacket")
		return
	}

	OnInterrupt(func() {
		pktCnn.Close()
	})

	// closeChan := registerCloseCnn0(pktCnn)
	rbuf := make([]byte, 128*1024)
	for {
		rsize, raddr, err := pktCnn.ReadFrom(rbuf)
		if err != nil {
			// logger.Error(err, "error ReadFrom")
			break
		}
		var oldUdpSsn *udpSession
		newUdpSsn := newUdpSsn(c, raddr, pktCnn)
		ssnKey := raddr.String()
		ssn, ok := udpSsnMap.Load(ssnKey)
		if !ok {
			if newUdpSsn == nil {
				// logger.Error(fmt.Errorf("none of valid Udp Session"),
				// 	"cannot found old udpSession and cannot setup udpSession")
				continue
			}
			udpCounter.Add(1)
			udpSsnMap.Store(ssnKey, newUdpSsn)
			oldUdpSsn = newUdpSsn
			// logger.Info("new connection pair",
			// 	"1From", raddr.String(), "1To", pktCnn.LocalAddr().String(),
			// 	"2From", newUdpSsn.ToCnn.LocalAddr().String(), "2To", newUdpSsn.ToCnn.RemoteAddr().String())
			wg.Add(1)
			go func(arg1 *udpSession) {
				forwardUDP(arg1)
				udpCounter.Sub(1)
				udpSsnMap.Delete(ssnKey)
				wg.Done()
			}(newUdpSsn)
		} else {
			oldUdpSsn = ssn.(*udpSession)
		}
		_, err = oldUdpSsn.ToCnn.Write(rbuf[:rsize])
		oldUdpSsn.WriteTime.Store(time.Now())
		if err != nil {
			// logger.Error(err, "error 2WriteTo")
			_ = oldUdpSsn.Close()
			udpCounter.Sub(1)
			udpSsnMap.Delete(ssnKey)
		}
	}
	// close(closeChan)
}
