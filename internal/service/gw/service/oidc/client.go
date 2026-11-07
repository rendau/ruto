package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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
// Simultaneous refreshes of one token in this process (several tabs that got a
// 401 at once) share one call to the provider: one that rotates refresh tokens
// would refuse all but the first. The call outlives a caller that gives up, so
// the others still get the answer.
func (s *Service) Refresh(
	ctx context.Context,
	tokenEndpoint string,
	client *authModel.JwtClient,
	refreshToken string,
) (*TokenResponse, error) {
	key := tokenEndpoint + "\x00" + client.Id + "\x00" + refreshToken
	rep, err, _ := s.refreshes.Do(key, func() (any, error) {
		form := url.Values{
			"grant_type":    {"refresh_token"},
			"refresh_token": {refreshToken},
		}
		return s.token(context.WithoutCancel(ctx), tokenEndpoint, client, form)
	})
	if err != nil {
		return nil, err
	}
	return rep.(*TokenResponse), nil
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
