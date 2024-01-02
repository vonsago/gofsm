package fsm

import (
	"context"
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
	resp := &senate.SyncTermResp{
		Vote: "",
		Term: 0,
		Role: 0,
	}
	ca, ok := s.regulation.Get(CacheRegular)
	if !ok {
		return resp, ErrNodeNotReady
	}
	n := ca.(map[string]*Node)[s.id]
	resp.Term = n.Term
	resp.Vote = n.Vote
	resp.Role = senate.Role(n.Role)

	s.nioch <- &Node{
		Id:     in.Id,
		Addr:   in.Addr,
		Vote:   in.Vote,
		Role:   Candidate,
		Term:   in.Term,
		AliveT: time.Now(),
	}

	return resp, nil
}

func (s *Server) HeartBeat(ctx context.Context, in *senate.HeartReq) (*senate.HeartResp, error) {
	s.nioch <- &Node{
		Id:       "",
		Addr:     "",
		Vote:     "",
		Role:     0,
		Term:     in.Term,
		LeaderId: in.LeaderId,
		AliveT:   time.Now(),
		conn:     nil,
	}

	resp := &senate.HeartResp{Ready: true}
	return resp, nil
}

func (s *Server) Ping(ctx context.Context, in *emptypb.Empty) (*senate.Pong, error) {
	resp := &senate.Pong{}
	if ca, ok := s.regulation.Get(CacheRegular); ok {
		n := ca.(map[string]*Node)[s.id]
		resp.Ready = true
		resp.Term = n.Term
		resp.Role = senate.Role(n.Role)
		resp.LeaderId = n.LeaderId
		return resp, nil
	} else {
		return resp, ErrNodeNotReady
	}
}
