package protorpc

import (
	"crypto/tls"
	"net"
	"sync/atomic"
)

type Server struct {
	listener ChannelListener
	handlers map[string]Handler
	ls       net.Listener
	stop     int32
}

func NewServer(listener ChannelListener) *Server {
	s := &Server{
		listener: listener,
		handlers: make(map[string]Handler),
	}
	return s
}

func (s *Server) HandleFunc(cmd string, handler func(*Channel, *Request) *Response) {
	if _, ok := s.handlers[cmd]; ok {
		panic("multi handler for command:" + cmd)
	}
	s.handlers[cmd] = HandlerFunc(handler)
}

func (s *Server) Handle(cmd string, handler Handler) {
	if _, ok := s.handlers[cmd]; ok {
		panic("multi handler for command:" + cmd)
	}
	s.handlers[cmd] = handler
}

func (s *Server) Serve(addr string) error {
	ls, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.ls = ls
	Logger.Info("start serve tcp:", addr)
	atomic.StoreInt32(&s.stop, 0)
	for {
		cc, err := ls.Accept()
		if err != nil {
			if atomic.LoadInt32(&s.stop) != 0 {
				return nil
			}
			return err
		}
		c := newChannel(s.handlers, s.listener)
		c.serve(cc)
	}
	return nil
}

func (s *Server) ServeTls(addr string, tlsCfg *tls.Config) error {
	ls, err := tls.Listen("tcp", addr, tlsCfg)
	if err != nil {
		return err
	}
	s.ls = ls
	Logger.Info("start serve tls:", addr)
	atomic.StoreInt32(&s.stop, 0)
	for {
		cc, err := ls.Accept()
		if err != nil {
			if atomic.LoadInt32(&s.stop) != 0 {
				return nil
			}
			return err
		}
		c := newChannel(s.handlers, s.listener)
		c.serve(cc)
	}
	return nil
}

func (s *Server) ServeTlsWithPem(addr string, pem, key []byte) error {
	cert, err := tls.X509KeyPair(pem, key)
	if err != nil {
		return err
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	return s.ServeTls(addr, tlsConfig)
}

func (s *Server) ServeTlsWithPemFile(addr, pem, key string) error {
	cert, err := tls.LoadX509KeyPair(pem, key)
	if err != nil {
		return err
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	return s.ServeTls(addr, tlsConfig)
}

func (s *Server) Stop() {
	atomic.StoreInt32(&s.stop, 1)
	s.ls.Close()
}
