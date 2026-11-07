package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
)

// Exchange trades the authorization code for tokens (RFC 6749 §4.1.3 with the
// PKCE verifier of RFC 7636). redirectUri must be the one the code was issued
// for.
func (s *Service) Exchange(
	ctx context.Context,
	tokenEndpoint string,
	client *authModel.JwtClient,
	code, codeVerifier, redirectUri string,
) (*TokenResponse, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectUri},
		"code_verifier": {codeVerifier},
	}
	return s.token(ctx, tokenEndpoint, client, form)
}

// Refresh trades a refresh token for a new token pair (RFC 6749 §6).
//
// Refreshes of one token in this process share one call to the provider:
// those that come while it is in flight (several tabs got a 401 at once) and,
// for RefreshGrace after it, those the browser sent before it got the new
// cookies. A provider that rotates refresh tokens would refuse all of them but
// the first. The call outlives a caller that gives up, so the others still
// get the answer. The shared answer must not be modified.
func (s *Service) Refresh(
	ctx context.Context,
	tokenEndpoint string,
	client *authModel.JwtClient,
	refreshToken string,
) (*TokenResponse, error) {
	key := tokenEndpoint + "\x00" + client.Id + "\x00" + refreshToken
	if rep := s.recentRefresh(key); rep != nil {
		return rep, nil
	}
	rep, err, _ := s.refreshes.Do(key, func() (any, error) {
		form := url.Values{
			"grant_type":    {"refresh_token"},
			"refresh_token": {refreshToken},
		}
		rep, err := s.token(context.WithoutCancel(ctx), tokenEndpoint, client, form)
		if err == nil {
			s.rememberRefresh(key, rep)
		}
		return rep, err
	})
	if err != nil {
		return nil, err
	}
	return rep.(*TokenResponse), nil
}

type recentRefresh struct {
	rep *TokenResponse
	at  time.Time
}

func (s *Service) recentRefresh(key string) *TokenResponse {
	s.recentMu.Lock()
	defer s.recentMu.Unlock()
	if item, ok := s.recent[key]; ok && time.Since(item.at) < RefreshGrace {
		return item.rep
	}
	return nil
}

// rememberRefresh keeps an answer for RefreshGrace; the outdated ones are
// dropped on the way, there are only as many as refreshes in that window.
func (s *Service) rememberRefresh(key string, rep *TokenResponse) {
	s.recentMu.Lock()
	defer s.recentMu.Unlock()
	now := time.Now()
	for k, item := range s.recent {
		if now.Sub(item.at) >= RefreshGrace {
			delete(s.recent, k)
		}
	}
	s.recent[key] = recentRefresh{rep: rep, at: now}
}

// token posts form to the token endpoint as a confidential client
// (client_secret_basic, the method every provider must support).
func (s *Service) token(
	ctx context.Context,
	tokenEndpoint string,
	client *authModel.JwtClient,
	form url.Values,
) (*TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	// RFC 6749 §2.3.1: the credentials are form-encoded before Basic.
	req.SetBasicAuth(url.QueryEscape(client.Id), url.QueryEscape(client.Secret))

	body, status, err := s.do(req)
	if err != nil {
		return nil, fmt.Errorf("token endpoint: %w", err)
	}

	if status < 200 || status >= 300 {
		refusal := &Error{Status: status}
		if err = json.Unmarshal(body, refusal); err != nil || refusal.Code == "" {
			return nil, fmt.Errorf("token endpoint: status %d, body: %.200s", status, body)
		}
		return nil, refusal
	}

	rep := new(TokenResponse)
	if err = json.Unmarshal(body, rep); err != nil {
		return nil, fmt.Errorf("token endpoint: json.Unmarshal: %w", err)
	}
	if rep.AccessToken == "" {
		return nil, fmt.Errorf("token endpoint: no access_token in the response")
	}
	return rep, nil
}
