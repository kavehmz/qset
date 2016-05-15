package qset

import (
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/kavehmz/crdt"
)

/*QSet is a race free implementation of what LWW can use as udnerlying set.
This implementation uses redis ZSET.
ZSET in redis uses scores to sort the elements. Score is a IEEE 754 floating point number,
that is able to represent precisely integer numbers between -(2^53) and +(2^53) included.
That is between -9007199254740992 and 9007199254740992.
This will limit this sets precision to save element's action timestamp to 1 milli-seconds.
Notice that time.Time precision is 1 nano-seconds by defaults. For this lack of precision all
timestamps are rounded to nearest microsecond.
Using redis can also cause latency cause by network or socket communication.
*/
type QSet struct {
	// Conn is the redis connection to be used.
	Conn redis.Conn
	// AddSet sets which key will be used in redis for the set.
	SetKey string
	// Marshal function needs to convert the lww.Element to string. Redis can only store and retrieve string values.
	Marshal func(lww.Element) string
	// UnMarshal function needs to be able to convert a Marshalled string back to a readable structure for consumer of library.
	UnMarshal func(string) lww.Element
	// LastState is an error type that will return the error state of last executed redis command. Add redis connection are not shareable this can be used after each command to know the last state.
	LastState error

	set lww.Set
	sync.RWMutex
}

type setData struct {
	element lww.Element
	ts      time.Time
}

func roundToMicro(t time.Time) int64 {
	return t.Round(time.Microsecond).UnixNano() / 1000
}

func (s *QSet) checkErr(err error) {
	if err != nil {
		s.LastState = err
		return
	}
	s.LastState = nil
}

//Init will do a one time setup for underlying set. It will be called from WLL.Init
func (s *QSet) Init() {
	if s.Conn == nil {
		s.checkErr(errors.New("Conn must be set"))
		return
	}
	if s.Marshal == nil {
		s.checkErr(errors.New("Marshal must be set"))
		return
	}
	if s.UnMarshal == nil {
		s.checkErr(errors.New("UnMarshal must be set"))
		return
	}
	if s.SetKey == "" {
		s.checkErr(errors.New("SetKey must be set"))
		return
	}
	_, err := s.Conn.Do("DEL", s.SetKey)
	s.checkErr(err)

	s.set.Init()
	s.readMembers()
}

func (s *QSet) readMembers() {
	zs, err := redis.Strings(s.Conn.Do("ZRANGE", s.SetKey, 0, -1, "WITHSCORES"))
	s.checkErr(err)
	for i := 0; i < len(zs); i += 2 {
		n, _ := strconv.Atoi(zs[i+1])
		s.set.Set(zs[i], time.Unix(0, 0).Add(time.Duration(n)*time.Microsecond))
	}
}

//Set adds an element to the set if it does not exists. It it exists Set will update the provided timestamp.
func (s *QSet) Set(e lww.Element, t time.Time) {
	s.set.Set(s.Marshal(e), t.Round(time.Microsecond))

	go func() {
		s.Lock()
		defer s.Unlock()
		_, err := s.Conn.Do("ZADD", s.SetKey, roundToMicro(t), s.Marshal(e))
		s.checkErr(err)
	}()
}

//Len must return the number of members in the set
func (s *QSet) Len() int {
	return s.set.Len()
}

//Get returns timestmap of the element in the set if it exists and true. Otherwise it will return an empty timestamp and false.
func (s *QSet) Get(e lww.Element) (time.Time, bool) {
	return s.set.Get(e)
}

//List returns list of all elements in the set
func (s *QSet) List() []lww.Element {
	var l []lww.Element
	for _, v := range s.set.List() {

		l = append(l, s.UnMarshal(v.(string)))
	}
	return l
}
