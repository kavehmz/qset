package qset

import (
	"testing"

	"github.com/garyburd/redigo/redis"
	"github.com/kavehmz/crdt"
)

func TestQSet_integration(t *testing.T) {
	var ac redis.Conn
	var asc redis.Conn
	var rc redis.Conn
	var rsc redis.Conn
	add := setupSet(t, &ac, &asc, "TESTADD")
	remove := setupSet(t, &rc, &rsc, "TESTREMOVE")

	lww.IntegrationTest(add, remove, t)
}
