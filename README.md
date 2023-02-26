# koron-go/redlock

[![GoDoc](https://godoc.org/github.com/koron-go/redlock?status.svg)](https://godoc.org/github.com/koron-go/redlock)
[![CircleCI](https://img.shields.io/circleci/project/github/koron-go/redlock/master.svg)](https://circleci.com/gh/koron-go/redlock/tree/master)
[![Go Report Card](https://goreportcard.com/badge/github.com/koron-go/redlock)](https://goreportcard.com/report/github.com/koron-go/redlock)

An experimental implementation of [Distributed Locks with Redis](https://redis.io/docs/manual/patterns/distributed-locks/)

## How to test

```
$ docker run --rm -it -p 6379:6379 --name mem-redis redis:7.0.8-alpine3.17
```

```
$ go test
```
