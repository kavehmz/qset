package qset

import (
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/kavehmz/crdt"
)

/*QSet a implementation of TimedSet for LWW that used Redis as its persistence later but Maps for operations.
This mix will make it about 100 times faster than original RedisSet.
This implementatin will have more memory foot print handle some operation in a non-blocking way.

Init function will initialize the internal map from redis. Also it will subscribe to a channel with the same name as SetKey to get the new changes and it will apply them to the map.
*/
type QSet struct {
	// Conn is the redis connection to be used.
	Conn redis.Conn
	// SetSet sets which key will be used in redis for the set.
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
