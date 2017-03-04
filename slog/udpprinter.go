package slog

import (
	"net"
	"time"
)

type _UdpNetPrinter struct {
	ch          chan *string
	blockMillis time.Duration
	conn        *net.UDPConn
}

func _NewUdpNetPrinter(addr string, maxrn int, blockMillis int) (_Printer, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	if blockMillis < 10 {
		blockMillis = 10
	}
	p := &_UdpNetPrinter{
		ch:          make(chan *string, maxrn),
		blockMillis: time.Millisecond * time.Duration(blockMillis),
	}
	p.conn, err = net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}
	t := time.Now()
	s := t.Format("[01-02 15:04:05.999][NOTE ] ------------ start ------------\n")
	p.Print(&s)
	go p.flush()
	return p, err

}

func (this *_UdpNetPrinter) Print(s *string) {
	select {
	case this.ch <- s:
	case <-time.After(this.blockMillis):
		es := "UdpPrinter enqueue timeout:" + *s
		slogerr(&es)
	}
}

func (this *_UdpNetPrinter) flush() {
	var bs []byte
	for v := range this.ch {
		bs = []byte(*v)
		this.conn.Write(bs)
	}
}
