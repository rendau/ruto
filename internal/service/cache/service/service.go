package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/goccy/go-json"

	"github.com/rendau/ruto/internal/service/cache"
)

type Service struct {
	repo      RepoI
	keyPrefix string
}

func New(repo RepoI, keyPrefix string) *Service {
	return &Service{
		repo:      repo,
		keyPrefix: keyPrefix,
	}
}

func (o *Service) NewChildInstance(prefix string) cache.CacheI {
	return &Service{
		keyPrefix: o.keyPrefix + prefix,
		repo:      o.repo,
	}
}

func (o *Service) Set(key string, value []byte, ttl time.Duration) error {
	err := o.repo.Set(o.keyPrefix+key, value, ttl)
	if err != nil {
		return fmt.Errorf("repo.Set: %w", err)
	}
	return nil
}

func (o *Service) Get(key string) ([]byte, bool, error) {
	data, found, err := o.repo.Get(o.keyPrefix + key)
	if err != nil {
		return nil, false, fmt.Errorf("repo.Get: %w", err)
	}

	return data, found, nil
}

func (o *Service) ListKeys() ([]string, error) {
	keys, err := o.repo.ListKeys(o.keyPrefix)
	if err != nil {
		return nil, fmt.Errorf("repo.ListKeys: %w", err)
	}

	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, strings.TrimPrefix(key, o.keyPrefix))
	}

	return result, nil
}

func (o *Service) SetJsonObj(key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	err = o.Set(key, data, ttl)
	if err != nil {
		return fmt.Errorf("o.Set: %w", err)
	}

	return nil
}

func (o *Service) GetJsonObj(key string, dst any) (bool, error) {
	dataRaw, ok, err := o.Get(key)
	if err != nil {
		return ok, fmt.Errorf("o.Get: %w", err)
	}
	if !ok {
		return false, nil
	}

	err = json.Unmarshal(dataRaw, dst)
	if err != nil {
		return false, fmt.Errorf("json.Unmarshal: %w", err)
	}

	return true, nil
}

func (o *Service) Del(key string) error {
	err := o.repo.Del(o.keyPrefix + key)
	if err != nil {
		return fmt.Errorf("repo.Del: %w", err)
	}
	return nil
}
