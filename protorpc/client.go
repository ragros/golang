package protorpc

import (
	"crypto/tls"
	"net"
	"time"
)

type Client struct {
	addr      string
	redial    time.Duration
	channel   *Channel
	tlsConfig *tls.Config
	handlers  map[string]Handler
	listener  clientListener
}

type clientListener struct {
	listener ChannelListener
	*Client
}

//if reDial is zero,no retry
func NewClient(svrAddr string, reDial int32, tlsCfg *tls.Config, listener ChannelListener) *Client {
	c := &Client{
		addr:      svrAddr,
		redial:    time.Duration(reDial) * time.Millisecond,
		tlsConfig: tlsCfg,
		handlers:  make(map[string]Handler),
	}
	c.listener.listener = listener
	c.listener.Client = c
	return c
}

//if connect success , return true
func (c *Client) Serve() bool {
	if c.channel != nil {
		c.channel.Close()
		<-time.After(100 * time.Millisecond)
	}
	c.channel = newChannel(c.handlers, &c.listener)
	return c.dialLoop()
}

func (c *Client) ServeBG() {
	if c.channel != nil {
		return
	}
	c.channel = newChannel(c.handlers, &c.listener)
	go c.dialLoop()
}

func (c *Client) Execute(req *Request, timeoutMills int) *Response {
	return c.channel.Execute(req, time.Duration(timeoutMills)*time.Millisecond)
}

func (c *Client) Notice(req *Request) (err error) {
	return c.channel.Notice(req)
}

func (c *Client) Handle(cmd string, h Handler) {
	if _, ok := c.handlers[cmd]; ok {
		panic("multi handler for command:" + cmd)
	}
	c.handlers[cmd] = h
}

func (c *Client) HandleFunc(cmd string, handler func(*Channel, *Request) *Response) {
	if _, ok := c.handlers[cmd]; ok {
		panic("multi handler for command:" + cmd)
	}
	c.handlers[cmd] = HandlerFunc(handler)
}

//if connect success , return true
func (c *Client) dialLoop() (succ bool) {
	var err error
	var conn net.Conn
	for {
		if c.tlsConfig != nil {
			conn, err = tls.Dial("tcp", c.addr, c.tlsConfig)
		} else {
			conn, err = net.Dial("tcp", c.addr)
		}
		if err == nil {
			break
		}
		errPrint("connect to server failed:", err)
		if c.redial == 0 {
			return
		}
		<-time.After(c.redial)
	}
	c.channel.serve(conn)
	succ = true
	return
}

func (c *Client) Close() {
	if c.channel != nil {
		c.channel.Close()
	}
}

func (c *clientListener) OnConnected(ch *Channel) {
	if c.listener != nil {
		c.listener.OnConnected(ch)
	}
}

func (c *clientListener) OnConnecting(ch *Channel) {
}

func (c *clientListener) OnDisconnect(ch *Channel) {
	if c.listener != nil {
		c.listener.OnDisconnect(ch)
	}
	if c.redial != 0 {
		go c.dialLoop()
	}
}

func (c *Client) IsValid() (ok bool) {
	if c.channel != nil {
		ok = c.channel.IsValid()
	}
	return
}
