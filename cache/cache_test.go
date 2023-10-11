package cache

import (
	"gotest.tools/assert"
	"sync"
	"testing"
	"time"
)

var ca *Cache

type Node struct {
	Val   int
	Left  *Node
	Right *Node
}

func TestCache(t *testing.T) {
	ca = New(-1, 0)
	ca.Set("tk", map[string]string{"k": "tv"}, time.Second*2)

	t.Run("testUpdateCache", testUpdateCache)

	time.Sleep(time.Second * 1)
	tk, found := ca.Get("tk")
	assert.Equal(t, found, true)
	if v, ok := tk.(map[string]string)["k"]; ok {
		assert.Equal(t, v, "vt")
	} else {
		panic("error update cache value")
	}
	time.Sleep(time.Second * 1)
	_, found = ca.Get("tk")
	assert.Equal(t, found, false)

	t.Run("testCacheOperation", testCacheOperation)
}

func testUpdateCache(t *testing.T) {
	tk, _ := ca.Get("tk")
	tk.(map[string]string)["k"] = "vt"
}

func testCacheOperation(t *testing.T) {
	k := "count"
	ca.Set(k, 0, -1)
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i += 1 {
		go func() {
			defer wg.Done()
			ca.OperationInt(k, "+", 1)
		}()
	}
	wg.Wait()
	count, _ := ca.Get(k)
	assert.Equal(t, count, 10)
}
