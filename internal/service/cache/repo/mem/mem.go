package mem

import (
	"strings"
	"sync"
	"time"
)

type item struct {
	value     []byte
	expiresAt time.Time
}

type Mem struct {
	mu    sync.RWMutex
	items map[string]item
}

func New() *Mem {
	return &Mem{
		items: make(map[string]item),
	}
}

func (o *Mem) Set(key string, value []byte, ttl time.Duration) error {
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	o.mu.Lock()
	o.items[key] = item{
		value:     value,
		expiresAt: expiresAt,
	}
	o.mu.Unlock()

	return nil
}

func (o *Mem) Get(key string) ([]byte, bool, error) {
	o.mu.RLock()
	data, ok := o.items[key]
	o.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}

	if !data.expiresAt.IsZero() && time.Now().After(data.expiresAt) {
		o.mu.Lock()
		current, ok := o.items[key]
		if ok && !current.expiresAt.IsZero() && time.Now().After(current.expiresAt) {
			delete(o.items, key)
		}
		o.mu.Unlock()
		return nil, false, nil
	}

	return data.value, true, nil
}

func (o *Mem) ListKeys(prefix string) ([]string, error) {
	now := time.Now()
	keys := make([]string, 0, len(o.items))
	expiredKeys := make([]string, 0, 8)

	o.mu.RLock()
	for key, data := range o.items {
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		if !data.expiresAt.IsZero() && now.After(data.expiresAt) {
			expiredKeys = append(expiredKeys, key)
			continue
		}
		keys = append(keys, key)
	}
	o.mu.RUnlock()

	if len(expiredKeys) > 0 {
		o.mu.Lock()
		now = time.Now()
		for _, key := range expiredKeys {
			current, ok := o.items[key]
			if !ok || current.expiresAt.IsZero() || !now.After(current.expiresAt) {
				continue
			}
			delete(o.items, key)
		}
		o.mu.Unlock()
	}

	return keys, nil
}

func (o *Mem) Del(key string) error {
	o.mu.Lock()
	delete(o.items, key)
	o.mu.Unlock()
	return nil
}
