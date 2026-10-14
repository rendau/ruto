package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/rendau/ruto/internal/domain/apikey/model"
	"github.com/rendau/ruto/internal/errs"
)

const (
	// KeyPrefix marks ruto keys so a stray key is recognizable in configs.
	KeyPrefix = "rtk_"

	keyRandomBytes = 32
	keyPrefixLen   = 12
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }

// GenerateKey returns a new key (shown to its owner once), its hash for
// storage and a short prefix to tell keys apart in lists.
func GenerateKey() (key, hash, prefix string, err error) {
	raw := make([]byte, keyRandomBytes)
	if _, err = rand.Read(raw); err != nil {
		return "", "", "", fmt.Errorf("rand.Read: %w", err)
	}

	key = KeyPrefix + hex.EncodeToString(raw)

	return key, HashKey(key), key[:keyPrefixLen], nil
}

// HashKey: keys are high-entropy random strings, so a plain sha256 without
// salt is enough.
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func (s *Service) List(ctx context.Context, pars *model.ListReq) ([]*model.ApiKey, int64, error) {
	items, tCount, err := s.repoDb.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("repoDb.List: %w", err)
	}
	return items, tCount, nil
}

func (s *Service) Get(ctx context.Context, id string, errNE bool) (*model.ApiKey, bool, error) {
	result, found, err := s.repoDb.Get(ctx, id)
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.Get: %w", err)
	}
	if !found {
		if errNE {
			return nil, false, errs.ObjectNotFound
		}
		return nil, false, nil
	}
	return result, true, nil
}

// GetByKey finds the key record for a presented key.
func (s *Service) GetByKey(ctx context.Context, key string) (*model.ApiKey, bool, error) {
	if !strings.HasPrefix(key, KeyPrefix) {
		return nil, false, nil
	}

	items, _, err := s.repoDb.List(ctx, &model.ListReq{KeyHash: new(HashKey(key))})
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.List: %w", err)
	}
	if len(items) == 0 {
		return nil, false, nil
	}
	return items[0], true, nil
}

func (s *Service) Create(ctx context.Context, obj *model.ApiKey) (string, error) {
	newId, err := s.repoDb.Create(ctx, obj)
	if err != nil {
		return "", fmt.Errorf("repoDb.Create: %w", err)
	}
	return newId, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repoDb.Delete(ctx, id); err != nil {
		return fmt.Errorf("repoDb.Delete: %w", err)
	}
	return nil
}

func (s *Service) TouchLastUsed(ctx context.Context, id string) error {
	if err := s.repoDb.TouchLastUsed(ctx, id); err != nil {
		return fmt.Errorf("repoDb.TouchLastUsed: %w", err)
	}
	return nil
}
