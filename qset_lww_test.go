package qset

import (
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/kavehmz/crdt"
)

func TestQSet_integration(t *testing.T) {
	var ac redis.Conn
	var asc redis.Conn
	var rc redis.Conn
	var rsc redis.Conn
	add := setupSetNoInit(t, &ac, &asc, "TESTADD")
	remove := setupSetNoInit(t, &rc, &rsc, "TESTREMOVE")

	lww := lww.LWW{AddSet: add, RemoveSet: remove}
	lww.Init()
	e := "e1"
	ts := time.Now()

	if lww.Exists(e) {
		t.Error("New LWW claims to containt an element")
	}

	lww.Add(e, ts)
	if !lww.Exists(e) {
		t.Error("Newly added element does not exists and it should")
	}

	ts = ts.Add(time.Second)
	lww.Remove(e, ts)
	if lww.Exists(e) {
		t.Error("An element which was remove with a more recent timestmap must be removed and is not")
	}

	ts = ts.Add(time.Second)
	lww.Add(e, ts)
	if !lww.Exists(e) {
		t.Error("An element which was remove and added again with a more recent timestamp does not exists")
	}
}
