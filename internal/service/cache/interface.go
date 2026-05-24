package cache

import (
	"time"
)

type CacheI interface {
	NewChildInstance(prefix string) CacheI
	Set(key string, value []byte, ttl time.Duration) error
	Get(key string) ([]byte, bool, error)
	ListKeys() ([]string, error)
	Del(key string) error
	SetJsonObj(key string, value any, ttl time.Duration) error
	GetJsonObj(key string, dst any) (bool, error)
}
