package qset

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
)

func TestQSet_init(t *testing.T) {
	r, _ := redis.Dial("tcp", ":6379")
	rs, _ := redis.Dial("tcp", ":6379")
	s0 := QSet{}
	s0.Init()
	if s0.LastState == nil {
		t.Error("No error for missing params")
	}

	s1 := QSet{ConnWrite: r}
	s1.Init()
	if s1.LastState == nil {
		t.Error("No error for missing params")
	}

	s2 := QSet{ConnWrite: r, ConnSub: rs}
	s2.Init()
	if s2.LastState == nil {
		t.Error("No error for missing params")
	}

	s3 := QSet{ConnWrite: r, ConnSub: rs, Marshal: func(e interface{}) string { return e.(string) }}
	s3.Init()
	if s3.LastState == nil {
		t.Error("No error for missing params")
	}

	s4 := QSet{ConnWrite: r, ConnSub: rs, Marshal: func(e interface{}) string { return e.(string) }, UnMarshal: func(e string) interface{} { return e }}
	s4.Init()
	if s4.LastState == nil {
		t.Error("No error for missing params")
	}

	s5 := QSet{ConnWrite: r, ConnSub: rs, Marshal: func(e interface{}) string { return e.(string) }, UnMarshal: func(e string) interface{} { return e }, SetKey: "TESTKEY"}
	s5.Init()
	if s5.LastState != nil {
		t.Error("Error raised when all params are present and correct")
	}
	s5.Set("test", time.Now())
	s5.Sync()

	s6 := QSet{ConnWrite: r, ConnSub: rs, Marshal: func(e interface{}) string { return e.(string) }, UnMarshal: func(e string) interface{} { return e }, SetKey: "TESTKEY"}
	s6.Init()
	if s6.Len() == 0 {
		t.Error("Was not able to load the previous data", s5.Len())
	}
}

func TestQSet_listenLoop(t *testing.T) {
	var r *redis.Conn
	var rs *redis.Conn
	s := setupSet(t, r, rs, "TESTKEY")
	defer s.Quit()

	time.Sleep(time.Millisecond)
	p, _ := redis.Dial("tcp", ":6379")
	p.Do("PUBLISH", "TESTKEY", "1463493139936983:data")
	time.Sleep(time.Second)

	if s.Len() != 1 || s.List()[0] != "data" {
		t.Error("Was not able to get data by subscribing")
	}

}

func setupSet(t interface {
	Error(...interface{})
}, r *redis.Conn, rs *redis.Conn, key string) *QSet {
	c1, err := redis.Dial("tcp", "localhost:6379")
	r = &c1
	if err != nil {
		t.Error("Can't setup redis for tests", err)
	}
	c1.Do("DEL", key)
	c2, err := redis.Dial("tcp", "localhost:6379")
	rs = &c2
	if err != nil {
		t.Error("Can't setup redis for tests", err)
	}
	s := QSet{ConnWrite: *r, ConnSub: *rs, Marshal: func(e interface{}) string { return e.(string) }, UnMarshal: func(e string) interface{} { return e }, SetKey: key}
	s.Init()
	return &s
}

func TestQSet(t *testing.T) {
	var r *redis.Conn
	var rs *redis.Conn
	s := setupSet(t, r, rs, "TESTKEY")
	defer s.Quit()

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
		t.Error("interface{} is not saved corretly", ts0, ok, ts)
	}

	ts = ts.Add(time.Second * 10)
	s.Set(a, ts)
	if ts0, ok := s.Get(a); !ok || ts0 != ts {
		t.Error("interface{} is not updated corretly")
	}

	s.Set("new data", ts)
	if ts0, ok := s.Get(a); !ok || ts0 != ts {
		t.Error("New interface{} is not added corretly")
	}

	l := s.List()
	if len(l) != 2 {
		t.Error("List is not returning all elements of the set correctly")
	}
	if l[0] != "data" || l[1] != "new data" {
		t.Error("List elements are not correct")
	}
	s.Sync()
}

func BenchmarkQSet_add_different_with_time_buffer_sync(b *testing.B) {
	var r *redis.Conn
	var rs *redis.Conn
	s := setupSet(b, r, rs, "TESTKEY")
	defer s.Quit()

	b.N = 1000
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Set(strconv.Itoa(i), time.Now())
	}
	s.Sync()
}

func BenchmarkQSet_add_different_with_time_buffer_no_sync(b *testing.B) {
	var r *redis.Conn
	var rs *redis.Conn
	s := setupSet(b, r, rs, "TESTKEY")
	defer s.Quit()

	b.N = 1000
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Set(strconv.Itoa(i), time.Now())
	}
	b.StopTimer()
	s.Sync()
}

func BenchmarkQSet_add_different(b *testing.B) {
	var r *redis.Conn
	var rs *redis.Conn
	s := setupSet(b, r, rs, "TESTKEY")
	defer s.Quit()

	b.N = 1000
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Set(strconv.Itoa(i), time.Now())
	}
	s.Sync()
}

func BenchmarkQSet_add_same(b *testing.B) {
	var r *redis.Conn
	var rs *redis.Conn
	s := setupSet(b, r, rs, "TESTKEY")
	defer s.Quit()

	b.N = 1000
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Set("test", time.Now().Add(time.Duration(i)*time.Microsecond))
	}
	s.Sync()
}

func BenchmarkQSet_get(b *testing.B) {
	var r *redis.Conn
	var rs *redis.Conn
	s := setupSet(b, r, rs, "TESTKEY")
	defer s.Quit()

	b.N = 1000
	for i := 0; i < b.N; i++ {
		s.Set(strconv.Itoa(i), time.Now())
	}
	s.Sync()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		s.Get(strconv.Itoa(i))
	}
}

func ExampleQSet() {
	c, _ := redis.Dial("tcp", "localhost:6379")
	s := QSet{ConnWrite: c, Marshal: func(e interface{}) string { return e.(string) }, UnMarshal: func(e string) interface{} { return e }, SetKey: "TESTKEY"}
	defer s.Quit()

	s.Init()
	s.Set("Data", time.Unix(1451606400, 0))
	ts, ok := s.Get("Data")
	fmt.Println(ok)
	fmt.Println(ts.Unix())
	// Output:
	// true
	// 1451606400
}
