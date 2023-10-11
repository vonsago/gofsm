package utils

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

type Pair[T, U any] struct {
	First  T
	Second U
}

type Ints interface {
	int | int8 | int16 | int32 | int64
}

type Uints interface {
	uint | uint8 | uint16 | uint32 | uint64
}

type Floats interface {
	float32 | float64
}

func Zip[T, U any](ts []T, us []U) []Pair[T, U] {
	// identify the minimum and maximum lengths
	lmin, lmax := len(ts), len(us)
	if lmin > lmax {
		lmax, lmin = len(ts), len(us)
	}
	pairs := make([]Pair[T, U], lmax)
	// build tuples up to the minimum length
	for i := 0; i < lmin; i++ {
		pairs[i] = Pair[T, U]{ts[i], us[i]}
	}
	if lmin == lmax {
		return pairs
	}
	// build tuples with one zero value for [lmin,lmax) range
	for i := lmin; i < lmax; i++ {
		p := Pair[T, U]{}
		if len(ts) == lmax {
			p.First = ts[i]
		} else {
			p.Second = us[i]
		}
		pairs[i] = p
	}
	return pairs
}

func Errorf(format string, a ...interface{}) error {
	return fmt.Errorf(fmt.Sprintf(format, a...))
}

func TimeCost(name string, start time.Time) {
	tc := time.Since(start)
	log.Infof("TimeCost %s time cost: %v", name, tc)
}

func GetEnv(key string, defaultVal interface{}) interface{} {
	if val, ok := os.LookupEnv(key); ok {
		switch defaultVal.(type) {
		case string:
			return val
		case int:
			if value, err := strconv.Atoi(val); err == nil {
				return value
			}
		case int64:
			if value, err := strconv.ParseInt(val, 10, 32); err == nil {
				return value
			}
		case float64:
			if value, err := strconv.ParseFloat(val, 64); err == nil {
				return value
			}
		case bool:
			if value, err := strconv.ParseBool(val); err == nil {
				return value
			}
		}
	}
	return defaultVal
}

func HashMD5(d string) (string, error) {
	h := md5.New()
	writeCnt, err := io.WriteString(h, d)
	if err != nil {
		return "", err
	}
	if writeCnt != len(d) {
		return "", Errorf("write count is not equal to length of source")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func unicodeStr(s string) string {
	CnSize := 3
	var b bytes.Buffer
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r > unicode.MaxLatin1 {
			if size > CnSize {
				r1, r2 := utf16.EncodeRune(r)
				b.WriteString(fmt.Sprintf("\\u%04x\\u%04x", r1, r2))
			} else {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			}
		} else {
			b.WriteRune(r)
		}
		s = s[size:]
	}
	return b.String()
}

func Decode64(s string) string {
	rawDecodedText, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(rawDecodedText)
}

func RandomPick[T any](seq []T, probabilities []float64) (t T) {
	// rand.Seed(time.Now().UnixNano())
	x := rand.Float64()
	cumulativeProbability := 0.0
	if len(seq) == 0 {
		return
	}
	rlt := seq[0]
	for _, p := range Zip(seq, probabilities) {
		cumulativeProbability += p.Second
		if x <= cumulativeProbability {
			rlt = p.First
			break
		}
	}
	return rlt
}

func CheckInside[T comparable](elem T, seq []T) bool {
	inside := false
	for _, e := range seq {
		if elem == e {
			inside = true
			break
		}
	}
	return inside
}

func Max[T Ints | Uints | Floats](e1 T, e2 T) T {
	if e1 > e2 {
		return e1
	}
	return e2
}

func Min[T Ints | Uints | Floats](e1 T, e2 T) T {
	if e1 > e2 {
		return e2
	}
	return e1
}

func Abs[T Ints | Uints | Floats](e T) T {
	if e < 0 {
		return -e
	}
	return e
}

func MapEq[T comparable](a, b map[string]T) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || v != w {
			return false
		}
	}
	return true
}

func GetAppRoot(appName string) string {
	r, err := os.Getwd()
	idx := strings.Index(r, appName)
	if err != nil || idx == -1 {
		return "/"
	}
	return r[:idx+len(appName)]
}

func Conv2Int(a any) (int, error) {
	switch t := a.(type) {
	case string:
		if v, err := strconv.ParseInt(a.(string), 10, 32); err == nil {
			return int(v), nil
		}
	case int32:
		return int(a.(int32)), nil
	case int64:
		return int(a.(int64)), nil
	case float64:
		return int(a.(float64)), nil
	case float32:
		return int(a.(float64)), nil
	default:
		return 0, fmt.Errorf("conv error type %t", t)
	}
	return 0, fmt.Errorf("conv error %v", a)
}

func SliceInterStr[T comparable](s1, s2 []T) []T {
	m := make(map[T]int)
	nn := make([]T, 0)
	for _, v := range s1 {
		m[v]++
	}
	for _, v := range s2 {
		if times, ok := m[v]; ok && times > 0 {
			nn = append(nn, v)
		}
	}
	return nn
}
