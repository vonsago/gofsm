package fsm

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	"github.com/vonsago/gofsm/api/senate"
	"github.com/vonsago/gofsm/cache"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"net"
	"strings"
	"sync"
	"time"
)

type RoleType int

const (
	Role = iota
	Leader
	Follower
	Candidate
)

const (
	CacheRegular = "cluster:regular"
)

type ClusterConf struct {
	Id         string
	Addrs      string
	Clusters   string
	Timeout    int32
	LeaderWork bool
	// cache result
	Regular *cache.Cache
	// cluster status
	Nodes map[string]*Node
	nlock sync.RWMutex
	stopc chan bool
	timer *time.Ticker
	// channel for server to self node
	eventc chan *Event
	nodec  chan *Node
	ctx    context.Context
}

type Node struct {
	Id       string
	Addr     string
	Vote     string
	Role     RoleType
	Term     int32
	LeaderId string
	AliveT   time.Time
	conn     *grpc.ClientConn
}

func NewClusterConf(id, ids, addrs string, timeout int32, work bool) *ClusterConf {
	nodes := make(map[string]*Node)
	idm := strings.Split(ids, ",")
	for i, v := range strings.Split(addrs, ",") {
		nodes[idm[i]] = &Node{
			Id:       idm[i],
			Addr:     v,
			Role:     Role,
			Vote:     "",
			Term:     0,
			LeaderId: "",
			AliveT:   time.Now(),
			conn:     nil,
		}
	}
	if nodes[id] == nil {
		panic(fmt.Sprintf("id %s of %s %s", id, addrs, ErrClusterInitError))
	}
	c := &ClusterConf{
		Id:         id,
		Addrs:      addrs,
		Timeout:    timeout,
		LeaderWork: work,
		Regular:    cache.New(cache.NoExpiration, cache.DefaultExpiration),
		Nodes:      nodes,
		timer:      time.NewTicker(1 * time.Second),
		eventc:     make(chan *Event),
		nodec:      make(chan *Node),
		ctx:        context.Background(),
	}
	return c
}

func (cf *ClusterConf) startServe() error {
	cf.nlock.RLock()
	ln := cf.Nodes[cf.Id]
	cf.nlock.RUnlock()
	listener, err := net.Listen("tcp", ln.Addr)
	if err != nil {
		log.Errorf("start serve at %s error: %v", ln.Addr, err)
		return err
	}
	s := grpc.NewServer()
	senate.RegisterLiveServer(s, &Server{
		id:         cf.Id,
		regulation: cf.Regular,
		nioch:      cf.nodec,
		eioch:      cf.eventc,
	})
	reflection.Register(s)
	go func() {
		err = s.Serve(listener)
		if err != nil {
			panic(err)
		}
	}()
	return nil
}

func (cf *ClusterConf) selfCheck() {
	cf.nlock.RLock()
	ln := cf.Nodes[cf.Id]
	ns := cf.Nodes
	cf.nlock.RUnlock()
	switch ln.Role {
	case Role:
		cf.nlock.Lock()
		cf.Nodes[cf.Id].Role = Follower
		cf.nlock.Unlock()
	case Leader:
		cf.SyncTerm2Others(ln, ns)
	case Follower:
		if int32(time.Now().Unix()-ln.AliveT.Unix()) > cf.Timeout {
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
			if cf.CheckClusterHealthy(ns) {

			}
		}
	}

	// cache nodes map for use
	cf.nlock.RLock()
	cf.Regular.Set(CacheRegular, cf.Nodes, cache.NoExpiration)
	cf.nlock.RUnlock()
}

func (cf *ClusterConf) GetOrConnectOthers(ns map[string]*Node) error {
	//var kacp = keepalive.ClientParameters{
	//	Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
	//	Timeout:             time.Second,      // wait 1 second for ping ack before considering the connection dead
	//	PermitWithoutStream: true,             // send pings even without active streams
	//}
	for _, v := range ns {
		if v.conn != nil || v.Id == cf.Id {
			continue
		}
		conn, err := grpc.Dial(
			v.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()),
			//grpc.WithKeepaliveParams(kacp),
		)
		if err != nil {
			log.Warningf("%s connect to %s at %s, error %v", cf.Id, v.Id, v.Addr, err)
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
	_ = cf.GetOrConnectOthers(ns)
	aliveCount := 1
	for _, v := range ns {
		if v.Id == cf.Id {
			continue
		}
		if v.conn == nil {
			log.Errorf("Connect from id[%s] to id[%s] fail, please check address: %s", cf.Id, v.Id, v.Addr)
			continue
		}
		c := senate.NewLiveClient(v.conn)
		resp, err := c.Ping(context.Background(), new(empty.Empty))
		if err != nil {
			log.Errorf("Heart from id[%s] to id[%s] at %s, error %v", cf.Id, v.Id, v.Addr, err)
			continue
		}

		if resp.Msg != "" {
			aliveCount++
		}
	}
	if aliveCount < 2 {
		log.Errorf("Alive Nodes Number: %d < 2, Cluster started failed", aliveCount)
		return false
	}
	return true
}

func (cf *ClusterConf) StartCampaign(ln *Node, ns map[string]*Node) bool {
	cf.nlock.Lock()
	defer cf.nlock.Unlock()

	_ = cf.GetOrConnectOthers(ns)
	getVotes := 0
	aliveCount := 0
	selfVotes := 0
	minId := ""
	req := &senate.SyncTermReq{
		Addr: ln.Addr,
		Id:   ln.Id,
		Role: Candidate,
		Vote: ln.Vote,
		Term: ln.Term + 1,
	}
	if ln.Vote == "" {
		ln.Vote = ln.Id
	}
	if ln.Vote == ln.Id {
		getVotes++
		aliveCount++
		selfVotes++
		minId = ln.Id
		for _, v := range ns {
			if v.conn == nil {
				continue
			}
			c := senate.NewLiveClient(v.conn)
			resp, err := c.CampaignLeader(cf.ctx, req)
			if err != nil || resp == nil {
				_ = v.conn.Close()
				v.conn = nil
				log.Warningf("canditate to %s at %s, error %v", ln.Id, v.Addr, err)
				continue
			}
			if resp.Term > ln.Term+1 {
				log.Warningf("candidate term %d is behand %d", ln.Term+1, resp.Term)
				return false
			}
			aliveCount++
			if resp.Vote == ln.Id {
				getVotes++
			}
			if resp.Vote == v.Id {
				selfVotes++
			}
			if minId == "" || minId > v.Id {
				minId = v.Id
			}
		}
	}
	if aliveCount == 0 {
		log.Warningf("all node died")
		if cf.LeaderWork {
			return true
		}
		return false
	}
	if getVotes > aliveCount-getVotes {
		log.Infof("id %s become leader at term %d get vote %d of %d", ln.Id, ln.Term+1, getVotes, aliveCount)
		return true
	}
	if aliveCount == selfVotes {
		log.Warningf("all alive node vote to self, %s revote to a min id node %s", ln.Id, minId)
		ln.Vote = minId
		return false
	}
	return false
}

func (cf *ClusterConf) SyncTerm2Others(ln *Node, ns map[string]*Node) {
	err := cf.GetOrConnectOthers(ns)
	if err != nil {
		log.Errorf("connect to other error %v", err)
	}
	for _, v := range ns {
		if v.conn == nil {
			continue
		}
		c := senate.NewLiveClient(v.conn)
		req := &senate.HeartReq{
			LeaderId: ln.Id,
			Term:     ln.Term,
		}
		resp, err := c.HeartBeat(cf.ctx, req)
		if err != nil {
			_ = v.conn.Close()
			v.conn = nil
			log.Errorf("leader: %s sync term to %s at %s error %v", cf.Id, v.Id, v.Addr, err)
		}
		if resp != nil {
			log.Debugf("leader: %s sync term to %s at %s ready %v", cf.Id, v.Id, v.Addr, resp.Ready)
		}
	}
}

func (cf *ClusterConf) passServeNode(n *Node) {
	if n == nil {
		return
	}
	cf.nlock.Lock()
	if n.LeaderId != "" {
		cf.Nodes[n.LeaderId].AliveT, cf.Nodes[cf.Id].AliveT = n.AliveT, n.AliveT
		cf.Nodes[n.LeaderId].Term, cf.Nodes[cf.Id].Term = n.Term, n.Term
		cf.Nodes[cf.Id].LeaderId = n.LeaderId
		cf.Nodes[cf.Id].Role = Follower
	} else if n.Role == Candidate {
		ln := cf.Nodes[cf.Id]
		if ln.Term > n.Term {
			log.Warningf("node: %s at term: %d is behand of local node: %s at term: %d", n.Id, n.Term, ln.Id, ln.Term)
		} else if ln.Vote == "" {
			ln.Vote = n.Vote
			cf.Nodes[n.Id].AliveT = n.AliveT
			cf.Nodes[n.Id].Term = n.Term
			cf.Nodes[n.Id].Vote = n.Vote
		}
	}
	cf.nlock.Unlock()
}

func (cf *ClusterConf) PutNode(id, addr string) error {
	cf.nlock.Lock()
	defer cf.nlock.Unlock()
	cf.Nodes[id] = &Node{
		Id:       id,
		Addr:     addr,
		Role:     Role,
		Vote:     "",
		Term:     0,
		LeaderId: id,
		AliveT:   time.Now(),
		conn:     nil,
	}
	return nil
}

func (cf *ClusterConf) Run() {
	err := cf.startServe()
	if err != nil {
		return
	}
	go func() {
		for {
			select {
			case n := <-cf.nodec:
				cf.passServeNode(n)
			}
		}
	}()
	for {
		select {
		case <-cf.stopc:
			log.Debug("stop cluster")
			return
		case <-cf.timer.C:
			cf.selfCheck()
		}
	}
}
