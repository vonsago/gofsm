package fsm

import (
	"testing"
)

func TestClusters(t *testing.T) {
	cls1 := NewClusterConf("1", "1,2,3", "0.0.0.0:8888,0.0.0.0:8887,0.0.0.0:8886", 10, false)
	cls2 := NewClusterConf("2", "1,2,3", "0.0.0.0:8888,0.0.0.0:8887,0.0.0.0:8886", 10, false)
	cls3 := NewClusterConf("3", "1,2,3", "0.0.0.0:8888,0.0.0.0:8887,0.0.0.0:8886", 10, false)
	go cls2.Run()
	go cls3.Run()

	cls1.Run()
}
