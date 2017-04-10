package simplebrd

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ragros/golang/protorpc"
)

type ChannelInfo struct {
	*protorpc.Channel
	group string
	id    string
}

func (c *ChannelInfo) Group() string {
	return c.group
}

func (c *ChannelInfo) Id() string {
	return c.id
}

func (c *ChannelInfo) String() string {
	return fmt.Sprintf("[%s:%s:%s]", c.group, c.id, c.Channel)
}

type listener struct {
	listener protorpc.ChannelListener
	*Server
}

type Server struct {
	mux     sync.RWMutex
	allchls []*ChannelInfo
	chlMap  map[*protorpc.Channel]*ChannelInfo
	idMap   map[string]*ChannelInfo
	grpMap  map[string][]*ChannelInfo

	lsr    listener
	change int32
	*protorpc.Server
}

func NewServer(listener protorpc.ChannelListener) *Server {
	s := &Server{
		chlMap: make(map[*protorpc.Channel]*ChannelInfo),
		idMap:  make(map[string]*ChannelInfo),
		grpMap: make(map[string][]*ChannelInfo),
	}
	s.lsr.listener = listener
	s.lsr.Server = s
	s.Server = protorpc.NewServer(&s.lsr)
	s.Server.HandleFunc("/simplepush_regist", s.onRegist)
	return s
}

func (s *listener) OnConnecting(c *protorpc.Channel) {
	s.mux.Lock()
	s.chlMap[c] = &ChannelInfo{Channel: c}
	s.mux.Unlock()
	atomic.StoreInt32(&s.change, 1)
}

func (s *listener) OnConnected(c *protorpc.Channel) {}

func (s *listener) OnDisconnect(c *protorpc.Channel) {
	s.mux.Lock()
	defer s.mux.Unlock()
	info, ok := s.chlMap[c]
	if !ok {
		return
	}
	delete(s.chlMap, c)
	if info.id != "" {
		delete(s.idMap, info.id)
	}
	if info.group != "" {
		gs, _ := s.grpMap[info.group]
		for i := 0; i < len(gs); i++ {
			if gs[i].Channel == c {
				gs[i] = nil
			}
		}
	}
	atomic.StoreInt32(&s.change, 1)
}

func (s *Server) Broadcast(req *protorpc.Request, exceptIds ...string) {
	bs, err := req.Marshal()
	if err != nil {
		protorpc.Logger.Error("multicast [%s] marshal failed:", req.Cmd(), err)
		return
	}
	chs := s.GetChannelAll()
	go func() {
		for _, ch := range chs {
			var found bool
			for _, id := range exceptIds {
				if ch.id == id {
					found = true
					break
				}
			}
			if found || !ch.IsValid() {
				continue
			}
			if err := ch.Send(bs); err != nil {
				protorpc.Logger.Warnf("broadcast [%s] to %s failed:%s", req.Cmd(), ch, err)
			}
		}
	}()
}

func (s *Server) Multicast(req *protorpc.Request, groups ...string) {
	if len(groups) == 0 {
		return
	}
	bs, err := req.Marshal()
	if err != nil {
		protorpc.Logger.Error("multicast [%s] marshal failed:", req.Cmd(), err)
		return
	}
	go func() {
		for _, g := range groups {
			gs, ok := s.GetChannelByGroup(g)
			if !ok {
				continue
			}
			for _, ch := range gs {
				if !ch.IsValid() {
					continue
				}
				if err := ch.Send(bs); err != nil {
					protorpc.Logger.Warnf("multicast [%s] to %s failed:%s", req.Cmd(), ch, err)
				}
			}
		}
	}()
}

func (s *Server) GetChannelAll() []*ChannelInfo {
	if atomic.CompareAndSwapInt32(&s.change, 1, 0) {
		s.mux.Lock()
		allchls := make([]*ChannelInfo, len(s.chlMap))
		var i int
		for _, v := range s.chlMap {
			allchls[i] = &ChannelInfo{Channel: v.Channel, id: v.id, group: v.group}
			i++
		}
		s.allchls = allchls
		s.mux.Unlock()
		return allchls
	} else {
		s.mux.RLock()
		allchls := s.allchls
		s.mux.RUnlock()
		return allchls
	}
}

func (s *Server) GetChannelById(id string) (ch *ChannelInfo, ok bool) {
	s.mux.RLock()
	ch, ok = s.idMap[id]
	s.mux.RUnlock()
	return
}

func (s *Server) GetChannelByGroup(group string) (gs []*ChannelInfo, ok bool) {
	s.mux.RLock()
	gs, ok = s.grpMap[group]
	s.mux.RUnlock()
	return
}

func (s *Server) onRegist(c *protorpc.Channel, r *protorpc.Request) (w *protorpc.Response) {
	group, err := r.GetString("group")
	if err != nil {
		w = protorpc.NewResponse(r.ReqId(), protorpc.Result_INVALID_REQUEST)
		return
	}
	id, err := r.GetString("id")
	if err != nil {
		w = protorpc.NewResponse(r.ReqId(), protorpc.Result_INVALID_REQUEST)
		return
	}
	s.mux.Lock()
	defer s.mux.Unlock()
	info, ok := s.chlMap[c]
	if !ok {
		return
	}
	atomic.StoreInt32(&s.change, 1)
	if id != "" {
		if _, ok = s.idMap[id]; ok {
			w = protorpc.NewResponse2(r.ReqId(), protorpc.Result_INVALID_REQUEST, "simplepush_regist_duplicate")
			return
		}
		s.idMap[id] = info
	}
	w = protorpc.NewResponse(r.ReqId(), protorpc.Result_OK)
	info.id = id
	info.group = group
	if info.group == "" {
		return
	}
	gs, ok := s.grpMap[group]
	if !ok {
		s.grpMap[group] = []*ChannelInfo{info}
		return
	}
	for i := 0; i < len(gs); i++ {
		if gs[i] == nil {
			gs[i] = info
			return
		}
	}
	s.grpMap[group] = append(gs, info)
	return
}
