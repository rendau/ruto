package oidc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
)

// Refreshes of one refresh token are shared by all gateway pods through Redis:
// the pod that takes the lock calls the provider and leaves the answer for
// RefreshGrace, the others wait for it. Redis is never on the path of an app
// request, only on the path of a refresh. When it fails, each pod refreshes on
// its own, as without it.
const (
	sharedKeyPrefix = "ruto:gw:refresh:"
	// sharedLockTTL outlives the provider call of the lock holder (bounded by
	// requestTimeout); a holder that died frees the lock by expiry.
	sharedLockTTL = requestTimeout + 2*time.Second
	// sharedPoll is how often a waiting pod looks for the answer.
	sharedPoll = 25 * time.Millisecond

	redisOpTimeout = time.Second
)

// SetRedis points the service at the Redis the pods share refreshes through;
// empty settings stop sharing. The previous client is closed.
func (s *Service) SetRedis(settings *snapshotModel.GatewaySettings) {
	var client *redis.Client
	if settings.RedisAddr != "" {
		client = redis.NewClient(&redis.Options{
			Addr:         settings.RedisAddr,
			Password:     settings.RedisPassword,
			DB:           settings.RedisDB,
			DialTimeout:  redisOpTimeout,
			ReadTimeout:  redisOpTimeout,
			WriteTimeout: redisOpTimeout,
			PoolTimeout:  redisOpTimeout,
			MaxRetries:   1,
		})
	}

	s.redisMu.Lock()
	previous := s.redis
	s.redis = client
	s.redisMu.Unlock()

	if previous != nil {
		_ = previous.Close()
	}
	if client != nil {
		slog.Info("oidc: refreshes are shared through redis", "addr", settings.RedisAddr, "db", settings.RedisDB)
	}
}

func (s *Service) redisClient() *redis.Client {
	s.redisMu.RLock()
	defer s.redisMu.RUnlock()
	return s.redis
}

// sharedRefresh returns the answer to a refresh of key, calling the provider
// with call only when no pod has done so within RefreshGrace and no pod is
// doing so now.
func (s *Service) sharedRefresh(client *redis.Client, key string, call func() (*TokenResponse, error)) (*TokenResponse, error) {
	answerKey := sharedAnswerKey(key)
	lockKey := answerKey + ":lock"
	deadline := time.Now().Add(sharedLockTTL)

	for {
		rep, err := readAnswer(client, answerKey)
		if err != nil {
			return unshared(err, call)
		}
		if rep != nil {
			return rep, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
		locked, err := client.SetNX(ctx, lockKey, 1, sharedLockTTL).Result()
		cancel()
		if err != nil {
			return unshared(err, call)
		}
		if locked {
			return refreshUnderLock(client, answerKey, lockKey, call)
		}

		// Another pod is calling the provider. A lock that outlived its
		// holder's call means the holder is gone: call the provider here.
		if time.Now().After(deadline) {
			return call()
		}
		time.Sleep(sharedPoll)
	}
}

// sharedAnswerKey hashes the refresh key: the refresh token stays out of the
// key names, which Redis tools and monitoring show.
func sharedAnswerKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return sharedKeyPrefix + hex.EncodeToString(sum[:])
}

func refreshUnderLock(client *redis.Client, answerKey, lockKey string, call func() (*TokenResponse, error)) (*TokenResponse, error) {
	rep, err := call()

	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()
	// A refusal is not kept: the waiting pods take the lock in turn and hear
	// it from the provider themselves.
	if err == nil {
		if raw, marshalErr := json.Marshal(rep); marshalErr == nil {
			if setErr := client.Set(ctx, answerKey, raw, RefreshGrace).Err(); setErr != nil {
				slog.Warn("oidc: redis: the refresh answer is not shared", "error", setErr)
			}
		}
	}
	if delErr := client.Del(ctx, lockKey).Err(); delErr != nil {
		slog.Warn("oidc: redis: refresh lock is left to expire", "error", delErr)
	}

	return rep, err
}

// readAnswer returns nil, nil when no pod has left an answer.
func readAnswer(client *redis.Client, answerKey string) (*TokenResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()

	raw, err := client.Get(ctx, answerKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rep := new(TokenResponse)
	if err = json.Unmarshal(raw, rep); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return rep, nil
}

func unshared(err error, call func() (*TokenResponse, error)) (*TokenResponse, error) {
	slog.Warn("oidc: redis is unavailable, refreshing without other pods", "error", err)
	return call()
}
