package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rendau/ruto/internal/domain/oauth/model"
	"github.com/rendau/ruto/internal/errs"
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }

func (s *Service) ListGrants(ctx context.Context, pars *model.GrantListReq) ([]*model.Grant, int64, error) {
	items, tCount, err := s.repoDb.ListGrants(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("repoDb.ListGrants: %w", err)
	}
	return items, tCount, nil
}

func (s *Service) GetGrant(ctx context.Context, id string, errNE bool) (*model.Grant, bool, error) {
	result, found, err := s.repoDb.GetGrant(ctx, id)
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.GetGrant: %w", err)
	}
	if !found {
		if errNE {
			return nil, false, errs.ObjectNotFound
		}
		return nil, false, nil
	}
	return result, true, nil
}

func (s *Service) CreateGrant(ctx context.Context, obj *model.Grant) (string, error) {
	newId, err := s.repoDb.CreateGrant(ctx, obj)
	if err != nil {
		return "", fmt.Errorf("repoDb.CreateGrant: %w", err)
	}
	return newId, nil
}

func (s *Service) DeleteGrant(ctx context.Context, id string) error {
	if err := s.repoDb.DeleteGrant(ctx, id); err != nil {
		return fmt.Errorf("repoDb.DeleteGrant: %w", err)
	}
	return nil
}

func (s *Service) MarkGrantConnected(ctx context.Context, id string) error {
	if err := s.repoDb.MarkGrantConnected(ctx, id); err != nil {
		return fmt.Errorf("repoDb.MarkGrantConnected: %w", err)
	}
	return nil
}

func (s *Service) TouchGrantLastUsed(ctx context.Context, id string) error {
	if err := s.repoDb.TouchGrantLastUsed(ctx, id); err != nil {
		return fmt.Errorf("repoDb.TouchGrantLastUsed: %w", err)
	}
	return nil
}

// IssueToken creates a token of the kind for the grant and returns it; only
// its hash is stored.
func (s *Service) IssueToken(ctx context.Context, grantId, kind string, ttl time.Duration) (string, error) {
	tokens, err := s.IssueTokens(ctx, grantId, map[string]time.Duration{kind: ttl})
	if err != nil {
		return "", err
	}
	return tokens[kind], nil
}

// IssueTokens creates one token per kind with its ttl, in a single insert.
func (s *Service) IssueTokens(ctx context.Context, grantId string, ttls map[string]time.Duration) (map[string]string, error) {
	now := time.Now()
	result := make(map[string]string, len(ttls))
	records := make([]*model.Token, 0, len(ttls))

	for kind, ttl := range ttls {
		token, hash, err := GenerateToken(kind)
		if err != nil {
			return nil, fmt.Errorf("GenerateToken: %w", err)
		}
		result[kind] = token
		records = append(records, &model.Token{
			Hash:      hash,
			GrantId:   grantId,
			Kind:      kind,
			ExpiresAt: now.Add(ttl),
		})
	}

	if err := s.repoDb.CreateTokens(ctx, records); err != nil {
		return nil, fmt.Errorf("repoDb.CreateTokens: %w", err)
	}

	return result, nil
}

// FindToken returns a live (not expired) token of the kind.
func (s *Service) FindToken(ctx context.Context, token, kind string) (*model.Token, bool, error) {
	item, found, err := s.repoDb.GetToken(ctx, HashToken(token))
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.GetToken: %w", err)
	}
	if !found || item.Kind != kind || !item.ExpiresAt.After(time.Now()) {
		return nil, false, nil
	}
	return item, true, nil
}

// FindAnyToken returns a token of any kind, expired or not.
func (s *Service) FindAnyToken(ctx context.Context, token string) (*model.Token, bool, error) {
	item, found, err := s.repoDb.GetToken(ctx, HashToken(token))
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.GetToken: %w", err)
	}
	return item, found, nil
}

// TakeToken consumes a single-use token (a code or a refresh token) and
// returns it if it was live.
func (s *Service) TakeToken(ctx context.Context, token, kind string) (*model.Token, bool, error) {
	item, found, err := s.repoDb.TakeToken(ctx, HashToken(token), kind)
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.TakeToken: %w", err)
	}
	if !found || !item.ExpiresAt.After(time.Now()) {
		return nil, false, nil
	}
	return item, true, nil
}

func (s *Service) Cleanup(ctx context.Context, grantsCreatedBefore time.Time) (tokens, grants int64, err error) {
	tokens, grants, err = s.repoDb.Cleanup(ctx, grantsCreatedBefore)
	if err != nil {
		return 0, 0, fmt.Errorf("repoDb.Cleanup: %w", err)
	}
	return tokens, grants, nil
}
