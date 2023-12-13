package fsm

import (
	log "github.com/sirupsen/logrus"
	"testing"
	"time"
)

func TestClusters(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cls1 := NewClusterConf("1", "1,2,3", "0.0.0.0:8888,0.0.0.0:8887,0.0.0.0:8886", 1, false)
	cls2 := NewClusterConf("2", "1,2,3", "0.0.0.0:8888,0.0.0.0:8887,0.0.0.0:8886", 1, false)
	cls3 := NewClusterConf("3", "1,2,3", "0.0.0.0:8888,0.0.0.0:8887,0.0.0.0:8886", 1, false)
	go cls1.Run()
	go cls2.Run()
	cls3.Run()

	time.Sleep(time.Second * 30)
	cls1.stopc <- true
	cls2.stopc <- true
	cls3.stopc <- true
}

func TestClusters3(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cls1 := NewClusterConf("1", "1,2,3", "0.0.0.0:8888,0.0.0.0:8887,0.0.0.0:8886", 5, false)
	cls1.Run()
}

func TestClusters1(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cls2 := NewClusterConf("2", "1,2,3", "0.0.0.0:8888,0.0.0.0:8887,0.0.0.0:8886", 5, false)
	cls2.Run()
}
func TestClusters2(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cls3 := NewClusterConf("3", "1,2,3", "0.0.0.0:8888,0.0.0.0:8887,0.0.0.0:8886", 5, false)
	cls3.Run()

}
