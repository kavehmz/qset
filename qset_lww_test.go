package qset

import (
	"testing"

	"github.com/garyburd/redigo/redis"
	"github.com/kavehmz/lww/integrate"
)

func TestQSet_integration(t *testing.T) {
	var ac redis.Conn
	var asc redis.Conn
	var rc redis.Conn
	var rsc redis.Conn
	add := setupSetNoInit(t, &ac, &asc, "TESTADD")
	remove := setupSetNoInit(t, &rc, &rsc, "TESTREMOVE")

	integrate.IntegrationTest(add, remove, t)
}
