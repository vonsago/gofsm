package fsm

import (
	log "github.com/sirupsen/logrus"
	"testing"
	"time"
)

func TestClusters1(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cls1 := NewClusterConf("1", "1,2,3", "0.0.0.0:8888,0.0.0.0:8887,0.0.0.0:8886", 5, false, nil)
	go cls1.Run()
	time.Sleep(time.Second * 30)
	cls1.cancel()
}

func TestClusters2(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cls2 := NewClusterConf("2", "1,2,3", "0.0.0.0:8888,0.0.0.0:8887,0.0.0.0:8886", 5, false, nil)
	go cls2.Run()
	time.Sleep(time.Second * 30)
	cls2.cancel()
}
func TestClusters3(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cls3 := NewClusterConf("3", "1,2,3", "0.0.0.0:8888,0.0.0.0:8887,0.0.0.0:8886", 5, false, nil)
	go cls3.Run()
	time.Sleep(time.Second * 30)
	cls3.cancel()
}

func TestClusterFsm(t *testing.T) {
	var ff *FSM
	if ff == nil {
		log.Infof("---")
	}
}
