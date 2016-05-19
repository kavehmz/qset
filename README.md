
[![Go Lang](http://kavehmz.github.io/static/gopher/gopher-front.svg)](https://golang.org/)
[![GoDoc](https://godoc.org/github.com/kavehmz/qset?status.svg)](https://godoc.org/github.com/kavehmz/qset)
[![Build Status](https://travis-ci.org/kavehmz/qset.svg?branch=master)](https://travis-ci.org/kavehmz/qset)
[![Coverage Status](https://coveralls.io/repos/kavehmz/qset/badge.svg?branch=master&service=github)](https://coveralls.io/github/kavehmz/qset?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/kavehmz/qset)](https://goreportcard.com/report/github.com/kavehmz/qset)


# qset
An underlying for LWW data structure. It is both fast and persistent.

It is an underlying set design for [LWW](https://godoc.org/github.com/kavehmz/qset)

QSet is an implementation of what LWW (lww.LWW) can use as udnerlying set.

It is for https://github.com/kavehmz/lww.

This implementation merges two approaches which are implemented in lww repositories to gain both speed and persistence at the same time.

It introduced a new underlying structure which each Set will add the element to a Go map (fast part) and write the element in redis in an async way (using ConnWrite connection). It will also publish the element to a channel in redis with the same name as SetKey.

It also subscribes to redis to a channel which the same name as SetKey (using ConnSub connection). Every time this or any other process publishes a new element this will update the internal map. This way it keeps the internal map up-to-date.

Converting data structure is done using Marshal and UnMarshal functions which must be provider by the user.

This implementation has the same time resolution limit as RedisSet that is minimum 1 millisecond.
