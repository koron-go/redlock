package redlock

import (
	"time"

	"github.com/go-redis/redis"
)

// cmdableAdapter is Adapter implementation for Cmdable of
// github.com/go-redis/redis package.
type cmdableAdapter struct {
	c redis.Cmdable
}

func (ca *cmdableAdapter) SetNX(key string, val string, expiration time.Duration) (bool, error) {
	return ca.c.SetNX(key, val, expiration).Result()
}

func (ca *cmdableAdapter) Eval(script string, key []string, arg string) error {
	return ca.c.Eval(script, key, arg).Err()
}

// NewWithRedis creates Mutex from some "github.com/go-redis/redis".Cmdable
// implementations.
func NewWithRedis(key string, clients ...redis.Cmdable) *Mutex {
	if len(clients) == 0 {
		panic("redlock requires one or more redis.Cmdable")
	}
	aa := make([]Adapter, len(clients))
	for i, c := range clients {
		aa[i] = &cmdableAdapter{c: c}
	}
	return New(key, aa...)
}
