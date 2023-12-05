package api

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/vonsago/gofsm/api/senate"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Server struct {
	senate.LiveServer
}

func (s *Server) Ping(ctx context.Context, in *emptypb.Empty) (*senate.Pong, error) {
	log.Debug()
	return &senate.Pong{Msg: "Hello "}, nil
}

func (s *Server) CampaignLeader(ctx context.Context, in *senate.SyncTermReq) (*senate.SyncTermResp, error) {
	return nil, nil
}

func (s *Server) HeartBeat(ctx context.Context, in *senate.HeartReq) (*senate.HeartResp, error) {
	return nil, nil
}
