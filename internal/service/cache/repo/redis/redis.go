package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

func New(url string, db int, psw string) *Redis {
	return &Redis{
		client: redis.NewClient(&redis.Options{
			Addr:     url,
			Password: psw,
			DB:       db,
		}),
	}
}

func (o *Redis) Set(key string, value []byte, ttl time.Duration) error {
	err := o.client.Set(context.Background(), key, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("client.Set: %w", err)
	}
	return nil
}

func (o *Redis) Get(key string) ([]byte, bool, error) {
	data, err := o.client.Get(context.Background(), key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("client.Get: %w", err)
	}

	return data, true, nil
}

func (o *Redis) ListKeys(prefix string) ([]string, error) {
	ctx := context.Background()
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
	err := o.client.Del(context.Background(), key).Err()
	if err != nil {
		return fmt.Errorf("client.Del: %w", err)
	}

	return nil
}
