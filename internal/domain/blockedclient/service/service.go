package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/rendau/ruto/internal/domain/blockedclient/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	"github.com/rendau/ruto/internal/errs"
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }

func (s *Service) List(ctx context.Context, pars *model.ListReq) ([]*model.BlockedClient, int64, error) {
	items, tCount, err := s.repoDb.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("repoDb.List: %w", err)
	}
	return items, tCount, nil
}

func (s *Service) Get(ctx context.Context, id string, errNE bool) (*model.BlockedClient, bool, error) {
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

// GetNotRemoved finds the block of the network that is not lifted (it may be
// expired); there is at most one.
func (s *Service) GetNotRemoved(ctx context.Context, ip string) (*model.BlockedClient, bool, error) {
	items, _, err := s.repoDb.List(ctx, &model.ListReq{Ip: &ip, NotRemoved: true})
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.List: %w", err)
	}
	if len(items) == 0 {
		return nil, false, nil
	}
	return items[0], true, nil
}

// CountActive counts blocks active at now, of one source if given.
func (s *Service) CountActive(ctx context.Context, now time.Time, source *string) (int64, error) {
	_, count, err := s.repoDb.List(ctx, &model.ListReq{ActiveAt: &now, Source: source, ListParams: listOnlyCount()})
	if err != nil {
		return 0, fmt.Errorf("repoDb.List: %w", err)
	}
	return count, nil
}

// CountCreatedSince counts blocks of a source created since the given time,
// whatever became of them.
func (s *Service) CountCreatedSince(ctx context.Context, source string, since time.Time) (int64, error) {
	_, count, err := s.repoDb.List(ctx, &model.ListReq{Source: &source, CreatedAfter: &since, ListParams: listOnlyCount()})
	if err != nil {
		return 0, fmt.Errorf("repoDb.List: %w", err)
	}
	return count, nil
}

// ListLiftedByAdmin returns blocks a person lifted after the given time.
func (s *Service) ListLiftedByAdmin(ctx context.Context, since time.Time) ([]*model.BlockedClient, error) {
	items, _, err := s.repoDb.List(ctx, &model.ListReq{
		RemovedSource: new(model.SourceAdmin),
		RemovedAfter:  &since,
		ListParams:    commonModel.ListParams{Sort: []string{"-removed_at", "id"}},
	})
	if err != nil {
		return nil, fmt.Errorf("repoDb.List: %w", err)
	}
	return items, nil
}

func (s *Service) RemoveMany(ctx context.Context, req *model.RemoveMany) (int64, error) {
	n, err := s.repoDb.RemoveMany(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("repoDb.RemoveMany: %w", err)
	}
	return n, nil
}

func (s *Service) Create(ctx context.Context, obj *model.BlockedClient) (string, error) {
	newId, err := s.repoDb.Create(ctx, obj)
	if err != nil {
		return "", fmt.Errorf("repoDb.Create: %w", err)
	}
	return newId, nil
}

func (s *Service) Update(ctx context.Context, id string, obj *model.Edit) error {
	if err := s.repoDb.Update(ctx, id, obj); err != nil {
		return fmt.Errorf("repoDb.Update: %w", err)
	}
	return nil
}

// Lock serializes writes to the block list until the ctx transaction ends.
func (s *Service) Lock(ctx context.Context) error {
	if err := s.repoDb.Lock(ctx); err != nil {
		return fmt.Errorf("repoDb.Lock: %w", err)
	}
	return nil
}

func (s *Service) CloseExpired(ctx context.Context, ip string, now time.Time) error {
	if err := s.repoDb.CloseExpired(ctx, ip, now); err != nil {
		return fmt.Errorf("repoDb.CloseExpired: %w", err)
	}
	return nil
}

// Active returns the blocks active at now with the version of that set.
func (s *Service) Active(ctx context.Context, now time.Time) ([]*model.Rule, string, error) {
	rules, err := s.repoDb.ActiveRules(ctx, now)
	if err != nil {
		return nil, "", fmt.Errorf("repoDb.ActiveRules: %w", err)
	}
	return rules, Version(rules), nil
}

func (s *Service) Stats(ctx context.Context, now, agentsSince time.Time) (*model.Stats, error) {
	result, err := s.repoDb.Stats(ctx, now, agentsSince)
	if err != nil {
		return nil, fmt.Errorf("repoDb.Stats: %w", err)
	}
	return result, nil
}

func (s *Service) Prune(ctx context.Context, endedBefore time.Time) (int64, error) {
	n, err := s.repoDb.Prune(ctx, endedBefore)
	if err != nil {
		return 0, fmt.Errorf("repoDb.Prune: %w", err)
	}
	return n, nil
}

// Version is computed from the active set itself rather than stored, so it
// also changes when a block expires — no write happens then. Expiry is hashed
// in unix seconds, as the gateways get it; rules come sorted by ip.
func Version(rules []*model.Rule) string {
	h := sha256.New()
	for _, rule := range rules {
		h.Write([]byte(rule.Ip))
		h.Write([]byte{' '})
		if rule.ExpiresAt != nil {
			h.Write(strconv.AppendInt(nil, rule.ExpiresAt.Unix(), 10))
		}
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func listOnlyCount() commonModel.ListParams {
	return commonModel.ListParams{OnlyCount: true}
}
