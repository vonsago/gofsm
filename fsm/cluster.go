package fsm

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	"github.com/vonsago/gofsm/api"
	"github.com/vonsago/gofsm/api/senate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"net"
	"strings"
	"sync"
	"time"
)

type ClusterConf struct {
	Id         string
	Addrs      string
	Clusters   string
	Timeout    int32
	LeaderWork bool
	// cluster status
	Nodes map[string]*Node
	nlock sync.RWMutex
	stopc chan bool
	timer *time.Timer
	// events
	eventc chan *Event
}

type RoleType int

const (
	Role = iota
	Leader
	Follower
	Candidate
)

type Node struct {
	Id       string
	Addr     string
	Vote     string
	Role     RoleType
	Ready    bool
	Term     int32
	LeaderId string
	AliveT   time.Time
	conn     *grpc.ClientConn
}

func NewClusterConf(id, addrs, clusters string, timeout int32, work bool) *ClusterConf {
	var nodes map[string]*Node
	for i, v := range strings.Split(clusters, ",") {
		d := fmt.Sprint(i)
		nodes[d] = &Node{
			Id:       d,
			Addr:     v,
			Role:     Role,
			Vote:     "",
			Ready:    false,
			Term:     0,
			LeaderId: d,
			AliveT:   time.Now(),
			conn:     nil,
		}
	}
	c := &ClusterConf{
		Id:         id,
		Addrs:      addrs,
		Timeout:    timeout,
		Clusters:   clusters,
		LeaderWork: work,
		Nodes:      nodes,
		timer:      time.NewTimer(1 * time.Second),
	}
	return c
}

func (cf *ClusterConf) startServe() error {
	listener, err := net.Listen("tcp", cf.Addrs)
	if err != nil {
		return err
	}
	server := grpc.NewServer()
	senate.RegisterLiveServer(server, &api.Server{})
	go func() {
		err = server.Serve(listener)
		if err != nil {
			panic(err)
		}
	}()
	return nil
}

func (cf *ClusterConf) run() {
	cf.nlock.RLock()
	ln := cf.Nodes[cf.Id]
	ns := cf.Nodes
	cf.nlock.RUnlock()
	if int32(time.Now().Unix()-ln.AliveT.Unix()) > cf.Timeout {
		cf.nlock.Lock()
		cf.Nodes[cf.Id].Ready = false
		cf.nlock.Unlock()
		log.Warningf("node %s at %s timeout", ln.Id, ln.Addr)
	}
	switch ln.Role {
	case Role:
		err := cf.startServe()
		if err != nil {
			log.Errorf("start serve at %s error %v", ln.Addr, err)
			return
		}
		cf.nlock.Lock()
		cf.Nodes[cf.Id].Role = Follower
		cf.Nodes[cf.Id].Ready = true
		cf.nlock.Unlock()
	case Leader:
		cf.SyncTerm2Others(ln, ns)
	case Follower:
		if ns[ln.LeaderId].Ready {
			if int32(time.Now().Unix()-ns[ln.LeaderId].AliveT.Unix()) > cf.Timeout {
				cf.nlock.Lock()
				cf.Nodes[ln.LeaderId].Ready = false
				cf.nlock.Unlock()
			}
		} else {
			if !cf.CheckClusterHealthy(ns) {
				return
			}
			cf.nlock.Lock()
			ns[cf.Id].Role = Candidate
			cf.nlock.Unlock()
		}
	case Candidate:
		if cf.StartCampaign(ln, ns) {
			cf.nlock.Lock()
			ns[cf.Id].Role = Leader
			ns[cf.Id].Term += 1
			cf.nlock.Unlock()
		} else {
			_ = cf.CheckClusterHealthy(ns)
		}
	}
	// TODO cache nodes map for use
}

func (cf *ClusterConf) GetOrConnectOthers(ns map[string]*Node) error {
	for _, v := range ns {
		if v.conn != nil || v.Id == cf.Id {
			continue
		}
		if int32(time.Now().Unix()-v.AliveT.Unix()) > cf.Timeout {
			// todo ping follower by leader
			log.Warningf("skip connect to %s at %s, latest time %v", v.Id, v.Addr, v.AliveT)
			continue
		}
		conn, err := grpc.Dial(v.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Warningf("connect to %s at %s, error %v", v.Id, v.Addr, err)
			continue
		}
		v.conn = conn
	}
	return nil
}

func (cf *ClusterConf) DisconnectAll(ns map[string]*Node) error {
	for _, v := range ns {
		if v.conn != nil {
			err := v.conn.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (cf *ClusterConf) CheckClusterHealthy(ns map[string]*Node) bool {
	err := cf.GetOrConnectOthers(ns)
	if err != nil {
		log.Errorf("connect to other error %v", err)
	}
	aliveCount := 1
	for _, v := range ns {
		c := senate.NewLiveClient(v.conn)
		resp, err := c.Ping(context.TODO(), new(empty.Empty))
		if err != nil {
			log.Warningf("heart to %s at %s, error %v", v.Id, v.Addr, err)
			continue
		}
		//if resp.Role == Leader && resp.Term > v.Term && resp.Ready {
		//	break
		//}
		if resp.Msg != "" {
			aliveCount++
		}
	}
	if aliveCount%2 == 0 {
		log.Warn("Even number of nodes cannot be started")
		return false
	}
	return true
}

func (cf *ClusterConf) StartCampaign(ln *Node, ns map[string]*Node) bool {
	getVotes := 1
	aliveCount := 1
	for _, v := range ns {
		if ln.conn == nil {
			continue
		}
		req := &senate.SyncTermReq{
			Addr: ln.Addr,
			Id:   ln.Id,
			Role: Candidate,
			Vote: ln.Addr,
			Term: ln.Term + 1,
		}
		c := senate.NewLiveClient(v.conn)
		resp, err := c.CampaignLeader(context.TODO(), req)
		if err != nil {
			log.Warningf("canditate to %s at %s, error %v", ln.Id, v.Addr, err)
			continue
		}
		if resp.Term > ln.Term+1 {
			log.Warningf("candidate term %d is behand %d", ln.Term, resp.Term)
			return false
		}
		if resp.Ready {
			aliveCount++
			if resp.Vote == ln.Id {
				getVotes++
			}
		}
	}
	if getVotes > aliveCount-getVotes {
		log.Infof("id %s become leader at term %d get vote %d of %d", ln.Id, ln.Term+1, getVotes, aliveCount)
		return true
	}
	return true
}

func (cf *ClusterConf) SyncTerm2Others(ln *Node, ns map[string]*Node) {
	err := cf.GetOrConnectOthers(ns)
	if err != nil {
		log.Errorf("connect to other error %v", err)
	}
	for _, v := range ns {
		c := senate.NewLiveClient(v.conn)
		req := &senate.HeartReq{
			LeaderId: ln.Id,
			Term:     ln.Term,
		}
		resp, err := c.HeartBeat(context.TODO(), req)
		if err != nil || !resp.Ready {
			log.Warningf("heart to %s at %s ready %v, error %v", v.Id, v.Addr, v.Ready, err)
		}
	}
}

func (cf *ClusterConf) Run() {
	for {
		select {
		case <-cf.stopc:
			return
		case <-cf.timer.C:
			cf.run()
		}
	}
}

func (cf *ClusterConf) PutNode(id, addr string) error {
	cf.nlock.Lock()
	defer cf.nlock.Unlock()
	cf.Nodes[id] = &Node{
		Id:       id,
		Addr:     addr,
		Role:     Role,
		Vote:     "",
		Ready:    false,
		Term:     0,
		LeaderId: id,
		AliveT:   time.Now(),
		conn:     nil,
	}
	return nil
}

func (cf *ClusterConf) LatestStatus(ready bool, role RoleType) {
	cf.nlock.RLock()
	defer cf.nlock.RUnlock()
	ln := cf.Nodes[cf.Id]
	ready = ln.Ready
	role = ln.Role
	return
}
