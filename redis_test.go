package redlock

import (
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis"
)

func newRedisMutex(t *testing.T, key, redisURL string) *Mutex {
	t.Helper()
	o, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("failed to ParseURL(%s): %s", redisURL, err)
	}
	c := redis.NewClient(o)
	return NewWithRedis(key, c)
}

func localRedisURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("TEST_REDIS_URL")
	if u != "" {
		t.Logf("using redis on %s", u)
		return u
	}
	return "redis://127.0.0.1:6379/0"
}

func newLocalRedisMutex(t *testing.T, key string, n int) []*Mutex {
	t.Helper()
	u := localRedisURL(t)
	r := make([]*Mutex, n)
	for i := 0; i < n; i++ {
		r[i] = newRedisMutex(t, key, u)
	}
	return r
}

func TestSingleLock(t *testing.T) {
	m := newLocalRedisMutex(t, "Lock", 2)
	err := m[0].Lock()
	if err != nil {
		t.Fatalf("lock#1 failed: %s", err)
	}
	defer m[0].Unlock()

	err = m[1].Lock()
	if err != ErrGaveUpLock {
		t.Fatalf("lock#2 unexpected: %s", err)
	}
}

func TestSingleUnlock(t *testing.T) {
	m := newLocalRedisMutex(t, "Unlock", 2)
	err := m[0].Lock()
	if err != nil {
		t.Fatalf("lock#1 failed: %s", err)
	}
	m[0].Unlock()

	err = m[1].Lock()
	if err != nil {
		t.Fatalf("lock#2 failed: %s", err)
	}
	m[1].Unlock()
}

func TestSingleExpire(t *testing.T) {
	m := newLocalRedisMutex(t, "Expire", 2)
	m[0].SetExpiration(500 * time.Millisecond)
	err := m[0].Lock()
	if err != nil {
		t.Fatalf("lock#1 failed: %s", err)
	}
	defer m[0].Unlock()

	err = m[1].Lock()
	if err != ErrGaveUpLock {
		t.Fatalf("lock#2-1 unexpected: %s", err)
	}
	time.Sleep(1000 * time.Millisecond)
	err = m[1].Lock()
	if err != nil {
		t.Fatalf("lock#2-2 failed: %s", err)
	}
	m[1].Unlock()
}
