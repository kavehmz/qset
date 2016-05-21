
[![Go Lang](http://kavehmz.github.io/static/gopher/gopher-front.svg)](https://golang.org/)
[![GoDoc](https://godoc.org/github.com/kavehmz/qset?status.svg)](https://godoc.org/github.com/kavehmz/qset)
[![Build Status](https://travis-ci.org/kavehmz/qset.svg?branch=master)](https://travis-ci.org/kavehmz/qset)
[![Coverage Status](https://coveralls.io/repos/kavehmz/qset/badge.svg?branch=master&service=github)](https://coveralls.io/github/kavehmz/qset?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/kavehmz/qset)](https://goreportcard.com/report/github.com/kavehmz/qset)


# qset

QSet is an implementation of what [LWW](https://github.com/kavehmz/lww) can use as its underlying set to provide a conflict-free replicated data type.

This implementation merges two approaches which are implemented in lww repositories to gain both speed and persistence at the same time.

It introduced a new underlying structure which each Set will add the element to a Go map (fast part) and write the element in redis in an async way. It will also publish the element to a channel in redis.

the flow after start is like:

- Subscribe to redis channel to get the latest changes and update the internal map.
- Read the persistent data from Redis. Because subscription to channel started first we dont miss the changes during this step.
- Set: Add the element to internal map and at the same time to redis and redis channel for other nodes to get the change.
- Get/Len/List: Only check the internal maps for asnwer.

Converting data structure is done using Marshal and UnMarshal functions which must be provider by the user.

This implementation has the same time resolution limit as RedisSet that is minimum 1 millisecond.
