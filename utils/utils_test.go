package utils

import (
	log "github.com/sirupsen/logrus"
	"gotest.tools/assert"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestHashMD5(t *testing.T) {
	dumpedBody := `{"field_id": "id1", "priority": 0, "slice": ["1500"], "map": {"k": "v", "k2": [1,2]}}`
	digest, err := HashMD5(dumpedBody)
	assert.Assert(t, err == nil)
	assert.Equal(t, digest, "d1dfbea37244db4f6cd182876dde4995")
}

func TestRandomPick(t *testing.T) {
	c := []string{"1", "2"}
	p := []float64{0.01, 0.99}
	c1, c2 := 0, 0
	RandomPick([]map[int]int{{1: 1}}, p)
	for i := 0; i < 100; i += 1 {
		r := RandomPick(c, p)
		if r == "1" {
			c1 += 1
		} else {
			c2 += 1
		}
	}
	log.Info(c1, c2)
	assert.Assert(t, c2 > c1)
	p = []float64{0, 1}
	for i := 0; i < 3; i += 1 {
		r := RandomPick(c, p)
		assert.Equal(t, r, "2")
	}
}

func TestZip(t *testing.T) {
	liA := []map[int]interface{}{{1: "a"}, {2: 0.3}}
	liB := []interface{}{"", 3.14, 998244353, map[string]string{"hello": "world"}}
	for i, p := range Zip(liA, liB) {
		log.Info(i, p)
	}
}

func TestGetEnv(t *testing.T) {
	_ = os.Setenv("testString", "test")
	env1 := GetEnv("testString", "test").(string)
	_ = os.Setenv("testString", "")
	assert.Equal(t, env1, "test")

	_ = os.Setenv("testInt", "333")
	env2 := GetEnv("testInt", 123).(int)
	_ = os.Setenv("testInt", "")
	assert.Equal(t, env2, 33)

	_ = os.Setenv("testFloat64", "1.999999999999999")
	env4 := GetEnv("testFloat64", 3.14).(float64)
	_ = os.Setenv("testFloat64", "")
	assert.Equal(t, env4, 1.999999999999999)

	_ = os.Setenv("testBool", "true")
	env5 := GetEnv("testBool", false).(bool)
	_ = os.Setenv("testBool", "")
	assert.Equal(t, env5, true)
}

func TestCheckInside(t *testing.T) {
	const N = 100000000
	rand.Seed(time.Now().UnixNano())
	arr := [N]int{}
	for i := 0; i < N; i++ {
		arr[i] = rand.Int()
	}
	ex := CheckInside(arr[N-1], arr[:])
	assert.Equal(t, ex, true)

	ex = CheckInside("golang", []string{"golang", "python", "c++"})
	assert.Equal(t, ex, true)

	ex = CheckInside(3.14, []float64{1.23, 4.56, 99.9, 3.1415926})
	assert.Equal(t, ex, false)
}

func TestConv2Int(t *testing.T) {
	x, err := Conv2Int(10.234)
	assert.Assert(t, err == nil)
	assert.Assert(t, x == 10)

	x, err = Conv2Int(int64(10))
	assert.Assert(t, err == nil)
	assert.Assert(t, x == 10)

	x, err = Conv2Int("10")
	assert.Assert(t, err == nil)
	assert.Assert(t, x == 10)

	x, err = Conv2Int(rune(10))
	assert.Assert(t, err == nil)
	assert.Assert(t, x == 10)
}

func TestSliceInside(t *testing.T) {
	s1 := []string{"aaa", "bbb"}
	s2 := []string{"aaa"}
	res := SliceInterStr(s1, s2)
	assert.Assert(t, reflect.DeepEqual([]string{"aaa"}, res))

	s3 := []string{"aaa", "bbb"}
	s4 := []string{"ccc"}
	res = SliceInterStr(s3, s4)
	assert.Assert(t, reflect.DeepEqual([]string{}, res))
}
