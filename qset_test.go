package qset

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/kavehmz/crdt"
)

func TestQSet_init(t *testing.T) {
	r, _ := redis.Dial("tcp", ":6379")
	s0 := QSet{}
	s0.Init()
	if s0.LastState == nil {
		t.Error("No error for missing params")
	}

	s1 := QSet{Conn: r}
	s1.Init()
	if s1.LastState == nil {
		t.Error("No error for missing params")
	}

	s2 := QSet{Conn: r, Marshal: func(e lww.Element) string { return e.(string) }}
	s2.Init()
	if s2.LastState == nil {
		t.Error("No error for missing params")
	}

	s3 := QSet{Conn: r, Marshal: func(e lww.Element) string { return e.(string) }, UnMarshal: func(e string) lww.Element { return e }}
	s3.Init()
	if s3.LastState == nil {
		t.Error("No error for missing params")
	}

	s4 := QSet{Conn: r, Marshal: func(e lww.Element) string { return e.(string) }, UnMarshal: func(e string) lww.Element { return e }, SetKey: "TESTKEY"}
	s4.Init()
	if s4.LastState != nil {
		t.Error("Error raised when all params are present and correct")
	}
}

func setupSet(t interface {
	Error(...interface{})
}, r *redis.Conn, key string) *QSet {
	c, err := redis.Dial("tcp", "localhost:6379")
	r = &c
	if err != nil {
		t.Error("Can't setup redis for tests", err)
	}
	s := QSet{Conn: *r, Marshal: func(e lww.Element) string { return e.(string) }, UnMarshal: func(e string) lww.Element { return e }, SetKey: key}
	s.Init()
	return &s
}

func TestQSet(t *testing.T) {
	var r *redis.Conn
	s := setupSet(t, r, "TESTKEY")

	if s.Len() != 0 {
		t.Error("New set if not empty")
	}

	a := "data"
	ts := time.Now().Round(time.Microsecond)
	s.Set(a, ts)
	if s.Len() != 1 {
		t.Error("Adding element to set failed")
	}
	if ts0, ok := s.Get(a); !ok || ts0 != ts {
		t.Error("lww.Element is not saved corretly", ts0, ok, ts)
	}

	ts = ts.Add(time.Second * 10)
	s.Set(a, ts)
	if ts0, ok := s.Get(a); !ok || ts0 != ts {
		t.Error("lww.Element is not updated corretly")
	}

	s.Set("new data", ts)
	if ts0, ok := s.Get(a); !ok || ts0 != ts {
		t.Error("New lww.Element is not added corretly")
	}

	l := s.List()
	if len(l) != 2 {
		t.Error("List is not returning all elements of the set correctly")
	}
	if l[0] != "data" || l[1] != "new data" {
		t.Error("List elements are not correct")
	}
}

func BenchmarkQSet_add_different(b *testing.B) {
	var r *redis.Conn
	s := setupSet(b, r, "TESTKEY")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		s.Set(strconv.Itoa(i), time.Now())
	}
}

func BenchmarkQSet_add_same(b *testing.B) {
	var r *redis.Conn
	s := setupSet(b, r, "TESTKEY")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		s.Set("test", time.Now().Add(time.Duration(i)*time.Microsecond))
	}
}

func BenchmarkQSet_get(b *testing.B) {
	var r *redis.Conn
	s := setupSet(b, r, "TESTKEY")
	for i := 0; i < b.N; i++ {
		s.Set(strconv.Itoa(i), time.Now())
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		s.Get(strconv.Itoa(i))
	}
}

func ExampleQSet() {
	c, _ := redis.Dial("tcp", "localhost:6379")
	s := QSet{Conn: c, Marshal: func(e lww.Element) string { return e.(string) }, UnMarshal: func(e string) lww.Element { return e }, SetKey: "TESTKEY"}
	s.Init()
	s.Set("Data", time.Unix(1451606400, 0))
	ts, ok := s.Get("Data")
	fmt.Println(ok)
	fmt.Println(ts.Unix())
	// Output:
	// true
	// 1451606400
}
