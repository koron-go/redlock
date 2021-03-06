package redlock

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	mathrand "math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Adapter defines requirements for redis connection.
type Adapter interface {
	SetNX(key string, val string, expiration time.Duration) (bool, error)
	Eval(script string, key []string, args ...interface{}) error
}

// Lock locks a key with id against an adapter.
func Lock(a Adapter, key, id string, expiration time.Duration) (bool, error) {
	ok, err := a.SetNX(key, id, expiration)
	if err != nil {
		return false, err
	}
	return ok, nil
}

const unlockScript = `
  if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
  else
    return 0
  end`

// Unlock unlocks a key with id against an adapter.
func Unlock(a Adapter, key, id string) error {
	err := a.Eval(unlockScript, []string{key}, id)
	if err != nil {
		return err
	}
	return nil
}

// adapters is collection of Adapter instances.
type adapters []Adapter

// lock tries to lock with all adapter.
func (aa adapters) lock(key, id string, expiration time.Duration) (int, []error) {
	var cnt int32
	var wg sync.WaitGroup
	var errs []error
	var mu sync.Mutex
	for _, a0 := range aa {
		wg.Add(1)
		go func(a Adapter) {
			ok, err := Lock(a, key, id, expiration)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
			if err == nil && ok {
				atomic.AddInt32(&cnt, 1)
			}
			wg.Done()
		}(a0)
	}
	wg.Wait()
	return int(cnt), errs
}

// unlock releases all locks
func (aa adapters) unlock(key, id string) {
	var wg sync.WaitGroup
	for _, a0 := range aa {
		wg.Add(1)
		go func(a Adapter) {
			Unlock(a, key, id)
			wg.Done()
		}(a0)
	}
	wg.Wait()
}

const idLen = 24 // 192 bits, birthday bound is 2^96.

// generateID generates random lock ID.
func generateID() (string, error) {
	b := make([]byte, idLen)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Mutex provides distributed locks with Redis (a.k.a. redlock).
// See https://redis.io/topics/distlock for details.
type Mutex struct {
	m sync.Mutex
	a adapters
	k string

	expiration time.Duration
	retryCount int
	retryDelay time.Duration

	lastID *string
}

var (
	// DefaultRetryCount is default max retry count to lock.
	DefaultRetryCount = 3

	// DefaultRetryDelay is default delay when retry to lock.
	DefaultRetryDelay = 200 * time.Millisecond

	// DefaultExpiration is default expiration time for Mutex's lock.
	DefaultExpiration = 30 * time.Second

	clockDriftFactor int64 = 100
)

// New creates a Mutex instance.
func New(key string, adapters ...Adapter) *Mutex {
	if len(adapters) == 0 {
		panic("redlock requires one or more Adapter")
	}
	return &Mutex{
		a:          adapters,
		k:          key,
		expiration: DefaultExpiration,
		retryCount: DefaultRetryCount,
		retryDelay: DefaultRetryDelay,
	}
}

// SetExpiration modifies expiration time of locked key.
func (m *Mutex) SetExpiration(ex time.Duration) {
	m.expiration = ex
}

func (m *Mutex) drift() time.Duration {
	return time.Duration(int64(m.expiration)/clockDriftFactor) +
		2*time.Millisecond
}

// SetRetryCount modifies max retry count to lock.
func (m *Mutex) SetRetryCount(n int) {
	m.retryCount = n
}

// SetRetryDelay modifies delay for retry to lock.
func (m *Mutex) SetRetryDelay(d time.Duration) {
	m.retryDelay = d
}

func (m *Mutex) randomDelay() time.Duration {
	return time.Duration(mathrand.Int63n(int64(m.retryDelay)))
}

var (
	// ErrLockedAlready occurs when previous lock is not released.
	ErrLockedAlready = errors.New("locked already")

	// ErrGaveUpLock ocurrs when Lock gave up.
	ErrGaveUpLock = errors.New("gave up lock")
)

// Lock tries to lock.
func (m *Mutex) Lock() error {
	m.m.Lock()
	defer m.m.Unlock()
	if m.lastID != nil {
		return ErrLockedAlready
	}
	id, err := generateID()
	if err != nil {
		return err
	}
	q := len(m.a)/2 + 1
	d0 := m.drift()
	for i := 0; i < m.retryCount; i++ {
		if i > 0 {
			time.Sleep(m.randomDelay())
		}
		st := time.Now()
		n, errs := m.a.lock(m.k, id, m.expiration)
		d := time.Since(st) + d0
		if len(errs) > 0 {
			m.a.unlock(m.k, id)
			return errs[0]
		}
		if n >= q && d < m.expiration {
			m.lastID = &id
			return nil
		}
		m.a.unlock(m.k, id)
	}
	return ErrGaveUpLock
}

// Unlock releases a lock.
func (m *Mutex) Unlock() {
	m.m.Lock()
	defer m.m.Unlock()
	if m.lastID == nil {
		return
	}
	m.a.unlock(m.k, *m.lastID)
	m.lastID = nil
}
