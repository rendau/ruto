package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
)

type legacyClient struct {
	baseURL      string
	refreshToken string
	httpClient   *http.Client
}

const (
	legacyReqTimeout     = 10 * time.Second
	legacyMaxAttempts    = 4
	legacyRetryBaseDelay = 200 * time.Millisecond
	legacyRetryMaxDelay  = 2 * time.Second
)

func newLegacyClient(baseURL, refreshToken string) *legacyClient {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: time.Second,
	}

	return &legacyClient{
		baseURL:      strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		refreshToken: strings.TrimSpace(refreshToken),
		httpClient: &http.Client{
			Timeout:   legacyReqTimeout,
			Transport: transport,
		},
	}
}

func (c *legacyClient) authByRefreshToken(ctx context.Context) (string, error) {
	rep := authByRefreshTokenRep{}
	err := c.requestJSON(ctx, http.MethodPost, legacyAPIPathAuthByRefreshToken, nil, authByRefreshTokenReq{
		RefreshToken: c.refreshToken,
	}, "", &rep)
	if err != nil {
		return "", fmt.Errorf("requestJSON: %w", err)
	}
	if strings.TrimSpace(rep.AccessToken) == "" {
		return "", fmt.Errorf("empty access_token")
	}
	return rep.AccessToken, nil
}

func (c *legacyClient) listRealms(ctx context.Context, accessToken string) ([]legacyRealm, error) {
	rep := paginatedListRep[legacyRealm]{}
	err := c.requestJSON(ctx, http.MethodGet, legacyAPIPathRealm, url.Values{}, nil, accessToken, &rep)
	if err != nil {
		return nil, fmt.Errorf("requestJSON: %w", err)
	}
	return rep.Results, nil
}

func (c *legacyClient) listApps(ctx context.Context, accessToken, realmID string) ([]legacyApp, error) {
	rep := paginatedListRep[legacyApp]{}
	err := c.requestJSON(ctx, http.MethodGet, legacyAPIPathApp, url.Values{
		"realm_id": []string{realmID},
	}, nil, accessToken, &rep)
	if err != nil {
		return nil, fmt.Errorf("requestJSON: %w", err)
	}
	return rep.Results, nil
}

func (c *legacyClient) listEndpoints(ctx context.Context, accessToken, appID string) ([]legacyEndpoint, error) {
	rep := paginatedListRep[legacyEndpoint]{}
	err := c.requestJSON(ctx, http.MethodGet, legacyAPIPathEndpoint, url.Values{
		"app_id": []string{appID},
	}, nil, accessToken, &rep)
	if err != nil {
		return nil, fmt.Errorf("requestJSON: %w", err)
	}
	return rep.Results, nil
}

func (c *legacyClient) fetchJWKKids(ctx context.Context, jwkURL string) ([]string, error) {
	rep := legacyJwkRep{}
	err := c.requestJSONByURL(ctx, http.MethodGet, strings.TrimSpace(jwkURL), nil, "", &rep)
	if err != nil {
		return nil, fmt.Errorf("requestJSONByURL: %w", err)
	}

	result := lo.FilterMap(rep.Keys, func(item legacyJwkKey, _ int) (string, bool) {
		kid := strings.TrimSpace(item.Kid)
		if kid == "" || !isAllowedJWKKey(item) {
			return "", false
		}
		return kid, true
	})
	return result, nil
}

func isAllowedJWKKey(item legacyJwkKey) bool {
	if use := strings.TrimSpace(item.Use); use != "" && !strings.EqualFold(use, "sig") {
		return false
	}

	alg := strings.TrimSpace(item.Alg)
	if alg == "" {
		return true
	}

	return constant.IsSupportedJWTAlgorithm(alg)
}

func (c *legacyClient) requestJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	reqBody any,
	accessToken string,
	repBody any,
) error {
	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}
	return c.requestJSONByURL(ctx, method, fullURL, reqBody, accessToken, repBody)
}

func (c *legacyClient) requestJSONByURL(
	ctx context.Context,
	method, fullURL string,
	reqBody any,
	accessToken string,
	repBody any,
) error {
	var bodyBytes []byte
	if reqBody != nil {
		var err error
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("json.Marshal: %w", err)
		}
	}

	var lastErr error

	for attempt := 1; attempt <= legacyMaxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("http.NewRequestWithContext: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if reqBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if accessToken != "" {
			req.Header.Set("Authorization", "Bearer "+accessToken)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("httpClient.Do: %w", err)
			if !shouldRetryOnErr(ctx, err) || attempt == legacyMaxAttempts {
				break
			}
			sleepWithContext(ctx, retryDelay(attempt, 0))
			continue
		}

		respBodyBytes, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("io.ReadAll: %w", readErr)
			if attempt == legacyMaxAttempts {
				break
			}
			sleepWithContext(ctx, retryDelay(attempt, 0))
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if repBody == nil || len(respBodyBytes) == 0 {
				return nil
			}
			if err = json.Unmarshal(respBodyBytes, repBody); err != nil {
				return fmt.Errorf("json.Unmarshal: %w", err)
			}
			return nil
		}

		lastErr = fmt.Errorf("bad status: %d, body: %s", resp.StatusCode, strings.TrimSpace(string(respBodyBytes)))
		if !shouldRetryOnStatus(resp.StatusCode) || attempt == legacyMaxAttempts {
			break
		}
		sleepWithContext(ctx, retryDelay(attempt, parseRetryAfter(resp.Header.Get("Retry-After"))))
	}

	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("request failed")
}

func shouldRetryOnErr(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	if _, ok := errors.AsType[net.Error](err); ok {
		return true
	}
	return true
}

func shouldRetryOnStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

func retryDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > legacyRetryMaxDelay {
			return legacyRetryMaxDelay
		}
		return retryAfter
	}

	delay := legacyRetryBaseDelay << (attempt - 1)
	if delay > legacyRetryMaxDelay {
		return legacyRetryMaxDelay
	}
	return delay
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	seconds, err := strconv.Atoi(v)
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func sleepWithContext(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
