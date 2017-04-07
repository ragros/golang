package protorpc

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"

	pb "github.com/ragros/golang/protorpc/internal"
)

var (
	helloReq               = []byte{0xAA, 0, 0, 0}
	helloRsp               = []byte{0xAA, 0, 0, 1}
	frameMask         byte = 0xAA
	HeartBeatDuration      = 180 * time.Second
)

type Channel struct {
	conn     net.Conn
	handlers map[string]Handler
	rspCh    map[string]chan *Response
	listener ChannelListener
	header   []byte
	valid    int32

	mux sync.Mutex
}

func newChannel(hs map[string]Handler, listener ChannelListener) *Channel {
	c := &Channel{
		rspCh:    make(map[string]chan *Response),
		listener: listener,
		handlers: hs,
		header:   make([]byte, 4),
	}
	return c
}

func (c *Channel) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

func (c *Channel) String() string {
	return fmt.Sprintf("[%s]", c.conn.RemoteAddr().String())
}

func (c *Channel) IsValid() bool {
	return atomic.LoadInt32(&c.valid) == 1
}

func (c *Channel) putRspChan(id string) (chan *Response, bool) {
	c.mux.Lock()
	if _, ok := c.rspCh[id]; ok {
		c.mux.Unlock()
		return nil, false
	}
	ch := make(chan *Response)
	c.rspCh[id] = ch
	c.mux.Unlock()
	return ch, true
}

func (c *Channel) remRspChan(id string) {
	c.mux.Lock()
	delete(c.rspCh, id)
	c.mux.Unlock()
}

func (c *Channel) getRspChan(id string) (ch chan *Response, ok bool) {
	c.mux.Lock()
	ch, ok = c.rspCh[id]
	c.mux.Unlock()
	return
}

func (c *Channel) Execute(req *Request, rspTimeout time.Duration) *Response {
	if atomic.LoadInt32(&c.valid) != 1 {
		return NewResponse(req.ReqId(), Result_LINK_BROKEN)
	}
	bs, err := req.Marshal()
	if err != nil {
		return NewResponse2(req.ReqId(), Result_CLIENT_EXCEPTION, err.Error())
	}
	ch, ok := c.putRspChan(req.ReqId())
	if !ok {
		return NewResponse(req.ReqId(), Result_DUPLICATE_REQID)
	}
	err = c.Send(bs)
	if err != nil {
		c.remRspChan(req.ReqId())
		return NewResponse2(req.ReqId(), Result_CLIENT_EXCEPTION, err.Error())
	}

	select {
	case m := <-ch:
		c.remRspChan(req.ReqId())
		return m
	case <-time.After(rspTimeout):
		c.remRspChan(req.ReqId())
		return NewResponse(req.ReqId(), Result_TIMEOUT)
	}
}

func (c *Channel) Send(bd []byte) error {
	if len(bd) > 0xffffff {
		return fmt.Errorf("frame over length")
	}
	dest := make([]byte, 4+len(bd))
	binary.BigEndian.PutUint32(dest, uint32(len(bd)))
	dest[0] = frameMask
	copy(dest[4:], bd)
	_, err := c.conn.Write(dest)
	if err != nil {
		c.conn.Close()
	}
	return err
}

func (c *Channel) Notice(req *Request) (err error) {
	if atomic.LoadInt32(&c.valid) != 1 {
		err = ErrLinkBroken
		return
	}
	var bs []byte
	bs, err = req.Marshal()
	if err != nil {
		return
	}
	err = c.Send(bs)
	return
}

func (c *Channel) writeResponse(rsp *Response) (err error) {
	if !c.IsValid() {
		err = ErrLinkBroken
		return
	}
	var bs []byte
	bs, err = rsp.marshal()
	if err != nil {
		return
	}
	err = c.Send(bs)
	return
}

func (c *Channel) readAtLeast(buf []byte, min int, timeout time.Time) (n int, err error) {
	if len(buf) < min {
		return 0, io.ErrShortBuffer
	}
	for n < min && err == nil {
		c.conn.SetReadDeadline(timeout)
		var nn int
		nn, err = c.conn.Read(buf[n:])
		n += nn
	}
	if n > 0 && err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return
}

func (c *Channel) readMessage() (body []byte, err error) {
	var timeout bool
	var lens int
	timeLimit := time.Now().Add(HeartBeatDuration)
	for {
		_, err = c.readAtLeast(c.header, 4, timeLimit)
		if err != nil {
			operr, ok := err.(*net.OpError)
			if !ok || !operr.Timeout() || timeout {
				return
			}
			if _, err = c.conn.Write(helloReq); err != nil {
				Logger.Error("failed hello request:", c.conn.RemoteAddr().String(), err)
			}
			timeout = true
			continue
		}
		if c.header[0] != frameMask {
			err = fmt.Errorf("invalid frame")
			return
		}
		c.header[0] = 0
		timeout = false
		lens = int(binary.BigEndian.Uint32(c.header))
		if lens == 0 {
			if _, err = c.conn.Write(helloRsp); err != nil {
				Logger.Warn("failed hello response:", c.conn.RemoteAddr().String(), err)
			}
			continue
		} else if lens == 1 {
			continue
		}
		break
	}
	body = make([]byte, lens)
	_, err = c.readAtLeast(body, lens, timeLimit)
	return
}

func (c *Channel) readLoop() {
	for {
		bs, err := c.readMessage()
		if err != nil {
			Logger.Error("channel read error:", err)
			break
		}
		go c.marshalAndHandle(bs)
	}
	c.conn.Close()
	atomic.StoreInt32(&c.valid, 0)
	if c.listener != nil {
		c.listener.OnDisconnect(c)
	}
}

func (c *Channel) marshalAndHandle(bs []byte) {
	msg := &pb.Message{}
	if err := proto.Unmarshal(bs, msg); err != nil {
		Logger.Warn("failed Unmarshal:", err)
		return
	}
	if req := msg.GetRequest(); req != nil {
		c.handle(req)
		return
	}
	if rsp := msg.GetResponse(); rsp != nil {
		if ch, ok := c.getRspChan(rsp.GetReqId()); ok {
			ack := &Response{msg: rsp}
			ack.dp = ack
			ch <- ack
		} else {
			Logger.Warnf("drop unknown response [%s]:%s", rsp.GetReqId(), c.String())
		}
	}

}

func (c *Channel) handle(req *pb.Request) {
	r := &Request{msg: req}
	r.dp = r
	var rsp *Response
	if h, ok := c.handlers[req.GetCmd()]; ok {
		rsp = h.Handle(c, r)
	} else {
		rsp = NewResponse2(r.ReqId(), Result_HANDLER_NOT_FOUND, req.GetCmd())
	}
	if rsp == nil {
		return
	}
	err := c.writeResponse(rsp)
	if err != nil {
		Logger.Error("write response error:", err)
	}
}

func (c *Channel) serve(con net.Conn) {
	c.conn = con
	if c.listener != nil {
		c.listener.OnConnecting(c)
	}
	go c.readLoop()
	atomic.StoreInt32(&c.valid, 1)
	if c.listener != nil {
		c.listener.OnConnected(c)
	}

}

func (c *Channel) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
