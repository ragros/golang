package simplebrd

import (
	"crypto/tls"
	"time"

	"github.com/ragros/golang/protorpc"
)

type clientListener struct {
	*Client
	listener protorpc.ChannelListener
}

type Client struct {
	group, id string
	*protorpc.Client
	lsr clientListener
}

func NewClient(svrAddr string, redialMillis int32, tlsCfg *tls.Config, listener protorpc.ChannelListener) *Client {
	c := &Client{}
	c.lsr.listener = listener
	c.lsr.Client = c
	c.Client = protorpc.NewClient(svrAddr, redialMillis, tlsCfg, &c.lsr)
	return c
}

func (c *Client) SetGroup(group string) *Client {
	c.group = group
	return c
}

func (c *Client) SetId(id string) *Client {
	c.id = id
	return c
}

func (c *clientListener) OnConnected(ch *protorpc.Channel) {
	if c.id != "" || c.group != "" {
		r := protorpc.NewRequest("/simplepush_regist")
		r.SetString("id", c.id)
		r.SetString("group", c.group)
		if rsp := c.Execute(r, 2000); !rsp.IsOK() {
			protorpc.Logger.Errorf("regist [%s,%s] failed:%s", c.group, c.id, rsp.String())
			c.Close()
			<-time.After(time.Second)
			return
		}
	}
	if c.listener != nil {
		c.listener.OnConnected(ch)
	}
}

func (c *clientListener) OnConnecting(ch *protorpc.Channel) {}

func (c *clientListener) OnDisconnect(ch *protorpc.Channel) {
	if c.listener != nil {
		c.listener.OnConnected(ch)
	}
}
