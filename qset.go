/*Package qset is an implementation of what LWW (https://github.com/kavehmz/lww) can use as its underlying set to provide a conflict-free replicated data type.

This implementation merges two approaches which are implemented in lww repositories to gain both speed and persistence at the same time.

It introduced a new underlying structure which each Set will add the element to a Go map (fast part) and write the element in redis in an async way. It will also publish the element to a channel in redis.

the flow after start is like:

  - Subscribe to redis channel to get the latest changes and update the internal map.
  - Read the persistent data from Redis. Because subscription to channel started first we dont miss the changes during this step.
  - Set: Add the element to internal map and at the same time to redis and redis channel for other nodes to get the change.
  - Get/Len/List: Only check the internal maps for asnwer.

Accessing data from redis has a time latency of about 50x more than accessing data from internal maps.
This mix of two methods can increase the access speed for mostly read system.

Converting data structure is done using Marshal and UnMarshal functions which must be provider by the user.

This implementation has the same time resolution limit as RedisSet that is minimum 1 millisecond.

*/
package qset

import (
	"errors"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/kavehmz/lww"
)

/*QSet structure defines the structure connects to Redis and needs two connections one for read and write and one for subscribing to channel.

QSet can only store data which is acceptable both as map key in Go and key name in Redis. Marshal function needs to make sure if this based on user data. UnMarshal must be able to convert the stored data back to a format that is usable by user.
*/
type QSet struct {
	// ConnWrite is the redis connection to be used for write elements to redis. This can be for example one master server.
	ConnWrite redis.Conn
	// ConnWrite is the redis connection to be used for subscribing to element notificatinos. This can be for example the local redis replica.
	ConnSub redis.Conn
	// AddSet sets which key will be used in redis for the set. Change will be also published in the channel with the same name.
	SetKey string
	// Marshal function needs to convert the element to string. Redis can only store and retrieve string values.
	Marshal func(interface{}) string
	// UnMarshal function needs to be able to convert a Marshalled string back to a readable structure for consumer of library.
	UnMarshal func(string) interface{}

	lastState error

	set lww.Set
	sync.WaitGroup
	sync.RWMutex

	setChannel chan setData
	sync       chan bool
	quit       chan bool

	// QueueMax set the buffer size for set channel. Larger numbers will icnrease the risk of losing data in case of crash but will create larger buffers that normally improve set performance.
	// When buffer is full, speed of each set will equal to speed of saving data in Redis.
	QueueMax  int
	setScript *redis.Script
	psc       redis.PubSubConn
}

type setData struct {
	element interface{}
	ts      time.Time
}

func roundToMicro(t time.Time) int64 {
	return t.Round(time.Microsecond).UnixNano() / 1000
}

func (s *QSet) checkErr(err error) {
	s.Lock()
	if err != nil {
		s.lastState = err
		s.Unlock()
		return
	}
	s.Unlock()
	s.lastState = nil
}

// LastState is an error type that will return the error state of last executed redis command. Add redis connection are not shareable this can be used after each command to know the last state.
func (s *QSet) LastState() error {
	s.Lock()
	st := s.lastState
	s.Unlock()
	return st
}

const updateToLatestAndPublishInRedis string = `
local c = tonumber(redis.call('ZSCORE', KEYS[1], ARGV[2]))
if not c or tonumber(ARGV[1]) > c then
	redis.call('ZADD', KEYS[1], ARGV[1], ARGV[2])
	redis.call('PUBLISH', KEYS[1], ARGV[1] .. ":" .. ARGV[2])
	return tonumber(ARGV[2])
else
	return 0
end
`

//Init will do a one time setup for underlying set. It will be called from WLL.Init
func (s *QSet) Init() {
	if !s.checkInitParams() {
		return
	}

	s.set.Init()

	listening := make(chan bool)
	go s.listenLoop(listening)
	<-listening
	s.readMembers()

	if s.QueueMax == 0 {
		s.QueueMax = 100000
	}
	s.setChannel = make(chan setData, s.QueueMax)
	s.sync = make(chan bool)
	s.quit = make(chan bool)

	//This Lua function will do a __atomic__ check and set of timestamp only in incremental way.
	s.setScript = redis.NewScript(1, updateToLatestAndPublishInRedis)

	go s.writeLoop()
}

func (s *QSet) checkInitParams() bool {
	if s.ConnWrite == nil {
		s.checkErr(errors.New("ConnWrite must be set"))
		return false
	}
	if s.ConnSub == nil {
		s.checkErr(errors.New("ConnSub must be set"))
		return false
	}
	if s.Marshal == nil {
		s.checkErr(errors.New("Marshal must be set"))
		return false
	}
	if s.UnMarshal == nil {
		s.checkErr(errors.New("UnMarshal must be set"))
		return false
	}
	if s.SetKey == "" {
		s.checkErr(errors.New("SetKey must be set"))
		return false
	}
	return true
}

func (s *QSet) listenLoop(listening chan bool) {
	s.psc = redis.PubSubConn{Conn: s.ConnSub}
	s.Lock()
	s.psc.Subscribe(s.SetKey)
	s.Unlock()
	listening <- true
	r := regexp.MustCompile(":")
	for {
		switch n := s.psc.Receive().(type) {
		case redis.Message:
			e := r.Split(string(n.Data), 2)
			tms, _ := strconv.Atoi(e[0])
			s.set.Set(s.UnMarshal(e[1]), time.Unix(0, 0).Add(time.Duration(tms)*time.Microsecond))
		case redis.Subscription:
			if n.Count == 0 {
				return
			}
		case error:
			s.checkErr(n)
			return
		}
	}
}

func (s *QSet) writeLoop() {
	for {
		select {
		case d := <-s.setChannel:
			s.setScript.Do(s.ConnWrite, s.SetKey, roundToMicro(d.ts), s.Marshal(d.element))
			s.Done()
		case <-s.quit:
			return
		}
	}
}

func (s *QSet) readMembers() {
	zs, err := redis.Strings(s.ConnWrite.Do("ZRANGE", s.SetKey, 0, -1, "WITHSCORES"))
	s.checkErr(err)
	for i := 0; i < len(zs); i += 2 {
		n, _ := strconv.Atoi(zs[i+1])
		s.set.Set(s.UnMarshal(zs[i]), time.Unix(0, 0).Add(time.Duration(n)*time.Microsecond))
	}
}

//Quit will end the write loop (Goroutine). This exist to be call at the end of Qset life to close the Goroutine to avoid memory leakage.
func (s *QSet) Quit() {
	s.quit <- true
	s.Lock()
	s.psc.Unsubscribe(s.SetKey)
	s.Unlock()
}

//Sync will block the call until redis queue is empty and all writes are done
func (s *QSet) Sync() {
	s.Wait()
}

//Set adds an element to the set if it does not exists. If it exists Set will update the provided timestamp. It also publishes the change into redis at SetKey channel.
func (s *QSet) Set(e interface{}, t time.Time) {
	s.set.Set(e, t.Round(time.Microsecond))
	s.Add(1)
	s.setChannel <- setData{ts: t.Round(time.Microsecond), element: e}
}

//Len must return the number of members in the set
func (s *QSet) Len() int {
	return s.set.Len()
}

//Get returns timestmap of the element in the set if it exists and true. Otherwise it will return an empty timestamp and false.
func (s *QSet) Get(e interface{}) (time.Time, bool) {
	return s.set.Get(e)
}

//List returns list of all elements in the set
func (s *QSet) List() []interface{} {
	var l []interface{}
	for _, v := range s.set.List() {
		l = append(l, v)
	}
	return l
}
