package service

import (
	"time"
)

type RepoI interface {
	Set(key string, value []byte, ttl time.Duration) error
	Get(key string) ([]byte, bool, error)
	ListKeys(prefix string) ([]string, error)
	Del(key string) error
}
