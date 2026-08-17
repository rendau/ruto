package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Timeouts are deliberately tight: the cache is auxiliary, so a stalled Redis
// must fail fast instead of holding up the callers. opTimeout caps a whole
// operation including retries, which the go-redis per-call timeouts alone do
// not do.
const (
	dialTimeout   = 2 * time.Second
	socketTimeout = 2 * time.Second
	opTimeout     = 3 * time.Second
	scanTimeout   = 5 * time.Second
	maxRetries    = 1
)

type Redis struct {
	client *redis.Client
}

func New(url string, db int, psw string) *Redis {
	return &Redis{
		client: redis.NewClient(&redis.Options{
			Addr:         url,
			Password:     psw,
			DB:           db,
			DialTimeout:  dialTimeout,
			ReadTimeout:  socketTimeout,
			WriteTimeout: socketTimeout,
			PoolTimeout:  socketTimeout,
			MaxRetries:   maxRetries,
		}),
	}
}

func (o *Redis) Set(key string, value []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	err := o.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("client.Set: %w", err)
	}
	return nil
}

func (o *Redis) Get(key string) ([]byte, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	data, err := o.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("client.Get: %w", err)
	}

	return data, true, nil
}

func (o *Redis) ListKeys(prefix string) ([]string, error) {
	// One budget for the whole cursor walk, not per SCAN call.
	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	defer cancel()

	pattern := prefix + "*"
	cursor := uint64(0)
	result := make([]string, 0, 64)
	seen := make(map[string]struct{}, 64)

	for {
		keys, nextCursor, err := o.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("client.Scan: %w", err)
		}
		for _, key := range keys {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, key)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return result, nil
}

func (o *Redis) Del(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	err := o.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("client.Del: %w", err)
	}

	return nil
}
