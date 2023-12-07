package fsm

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/vonsago/gofsm/api/senate"
	"github.com/vonsago/gofsm/cache"
	"google.golang.org/protobuf/types/known/emptypb"
	"time"
)

type Server struct {
	senate.LiveServer
	id         string
	regulation *cache.Cache
	nioch      chan *Node
	eioch      chan *Event
}

type RmtMsg struct {
}

func (s *Server) CampaignLeader(ctx context.Context, in *senate.SyncTermReq) (*senate.SyncTermResp, error) {
	return nil, nil
}

func (s *Server) HeartBeat(ctx context.Context, in *senate.HeartReq) (*senate.HeartResp, error) {
	s.nioch <- &Node{
		Id:       "",
		Addr:     "",
		Vote:     "",
		Role:     0,
		Ready:    true,
		Term:     in.Term,
		LeaderId: in.LeaderId,
		AliveT:   time.Now(),
		conn:     nil,
	}

	resp := &senate.HeartResp{}

	if rule, found := s.regulation.Get(CacheRegular); found {
		nodes := rule.(map[string]*Node)
		resp.Ready = nodes[s.id].Ready
		return resp, nil
	}
	return resp, nil
}

func (s *Server) Ping(ctx context.Context, in *emptypb.Empty) (*senate.Pong, error) {
	log.Debug()
	return &senate.Pong{Msg: "Hello "}, nil
}
