package fsm

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/vonsago/gofsm/api/senate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"gotest.tools/assert"
	"net"
	"testing"
)

func TestApi(t *testing.T) {
	addr := "0.0.0.0:8000"
	listener, err := net.Listen("tcp", addr)
	assert.Assert(t, err == nil)
	server := grpc.NewServer()
	senate.RegisterLiveServer(server, &Server{})
	go func() {
		err = server.Serve(listener)
		if err != nil {
			panic(err)
		}
	}()

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	c := senate.NewLiveClient(conn)
	resp, err := c.Ping(context.TODO(), &emptypb.Empty{})
	assert.Assert(t, err == nil)
	log.Infof("test Ping: %s", resp.Msg)
}
