package qset

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
)

func TestQSet_init(t *testing.T) {
	s0 := QSet{}
	s0.Init()
	if s0.LastState() == nil {
		t.Error("No error for missing params")
	}

	r, _ := redis.Dial("tcp", ":6379")
	rs, _ := redis.Dial("tcp", ":6379")

	s1 := QSet{ConnWrite: r}
	if s1.checkInitParams() {
		t.Error("No error for missing params")
	}

	s2 := QSet{ConnWrite: r, ConnSub: rs}
	if s2.checkInitParams() {
		t.Error("No error for missing params")
	}

	s3 := QSet{ConnWrite: r, ConnSub: rs, Marshal: func(e interface{}) string { return e.(string) }}
	if s3.checkInitParams() {
		t.Error("No error for missing params")
	}

	s4 := QSet{ConnWrite: r, ConnSub: rs, Marshal: func(e interface{}) string { return e.(string) }, UnMarshal: func(e string) interface{} { return e }}
	if s4.checkInitParams() {
		t.Error("No error for missing params")
	}

	s5 := QSet{ConnWrite: r, ConnSub: rs, Marshal: func(e interface{}) string { return e.(string) }, UnMarshal: func(e string) interface{} { return e }, SetKey: "TESTKEY"}
	s5.Init()
	if !s5.checkInitParams() {
		t.Error("Error raised when all params are present and correct")
	}
	s5.Set("test", time.Now())
	s5.Sync()
	s5.Quit()

	r, _ = redis.Dial("tcp", ":6379")
	rs, _ = redis.Dial("tcp", ":6379")
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

	s.psc.Close()
	time.Sleep(time.Millisecond * 100)
	if s.LastState() == nil {
		t.Error("Closing connection did not cause error in listenLoop")
	}
}
func setupSetNoInit(t testing.TB, r *redis.Conn, rs *redis.Conn, key string) *QSet {
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
	return &s
}

func setupSet(t testing.TB, r *redis.Conn, rs *redis.Conn, key string) *QSet {
	s := setupSetNoInit(t, r, rs, key)
	s.Init()
	return s
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

type p struct {
	Name string
	Age  int
}

func TestQSet_strcuture(t *testing.T) {
	var r0 *redis.Conn
	var rs0 *redis.Conn
	s0 := setupSet(t, r0, rs0, "TESTKEY")

	var r1 *redis.Conn
	var rs1 *redis.Conn
	s1 := setupSet(t, r1, rs1, "TESTKEY")

	s0.Marshal = func(e interface{}) string {
		b, _ := json.Marshal(e.(p))
		return string(b)
	}
	s0.UnMarshal = func(v string) interface{} {
		var e p
		json.Unmarshal([]byte(v), &e)
		return e
	}

	s1.Marshal = s0.Marshal
	s1.UnMarshal = s0.UnMarshal

	a := p{"name", 20}
	ts := time.Now().Round(time.Microsecond)
	s0.Set(a, ts)
	s0.Sync()
	time.Sleep(time.Second)

	// Not the s1 set should have the element in s0 set.
	if e, ok := s1.Get(a); !ok || e != ts {
		t.Error("Can't find  element added to set s0 in s1", ts, e, ok)
	}
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
	connWrite, _ := redis.Dial("tcp", "localhost:6379")
	connSub, _ := redis.Dial("tcp", "localhost:6379")
	s := QSet{ConnWrite: connWrite, ConnSub: connSub, Marshal: func(e interface{}) string { return e.(string) }, UnMarshal: func(e string) interface{} { return e }, SetKey: "TESTKEY"}
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
