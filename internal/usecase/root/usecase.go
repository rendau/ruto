package root

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
	"github.com/rendau/ruto/internal/domain/root/model"
	variableModel "github.com/rendau/ruto/internal/domain/variable/model"
	"github.com/rendau/ruto/internal/errs"
)

type Usecase struct {
	svc        ServiceI
	sessionSvc SessionServiceI
	httpClient *http.Client
}

func New(srv ServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:        srv,
		sessionSvc: sessionSvc,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (u *Usecase) Get(ctx context.Context) (*model.Root, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return nil, errs.NotAuthorized
	}

	result, err := u.svc.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	return result, nil
}

func (u *Usecase) Set(ctx context.Context, obj *model.Root) error {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return errs.NotAuthorized
	}

	if err := u.svc.Set(ctx, obj); err != nil {
		return fmt.Errorf("svc.Set: %w", err)
	}
	return nil
}

func (u *Usecase) GetVariablesEffective(ctx context.Context, variables []variableModel.Variable) ([]variableModel.Variable, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return nil, errs.NotAuthorized
	}

	variables, err := variableModel.NormalizeList(variables)
	if err != nil {
		return nil, fmt.Errorf("variables: %w", err)
	}
	result, err := variableModel.ResolveList(variables)
	if err != nil {
		return nil, fmt.Errorf("variables: %w", err)
	}
	return result, nil
}

func (u *Usecase) GetJwtKidsByURLs(ctx context.Context, urls []string) ([]string, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return nil, errs.NotAuthorized
	}

	return u.getJwtKidsByURLs(ctx, urls), nil
}

func (u *Usecase) getJwtKidsByURLs(ctx context.Context, urls []string) []string {
	jwkURLs := lo.FilterMap(urls, func(rawURL string, _ int) (string, bool) {
		jwkURL := strings.TrimSpace(rawURL)
		return jwkURL, jwkURL != ""
	})
	jwkURLs = lo.Uniq(jwkURLs)

	kidsByURL := lo.Map(jwkURLs, func(jwkURL string, _ int) []string {
		kids, loadErr := u.loadJwkKids(ctx, jwkURL)
		if loadErr != nil {
			return nil
		}
		return kids
	})

	result := lo.Uniq(lo.Flatten(kidsByURL))
	sort.Strings(result)
	return result
}

func (u *Usecase) loadJwkKids(ctx context.Context, jwkURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwkURL, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequestWithContext: %w", err)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.Client.Do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		repBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad status code: %d, body: %s", resp.StatusCode, string(repBody))
	}

	var repObj jwkResponse
	if err = json.NewDecoder(resp.Body).Decode(&repObj); err != nil {
		return nil, fmt.Errorf("json.Decode: %w", err)
	}

	result := lo.FilterMap(repObj.Keys, func(key jwkKey, _ int) (string, bool) {
		kid := strings.TrimSpace(key.Kid)
		return kid, kid != "" && constant.IsSupportedJWTAlgorithm(key.Alg)
	})

	return result, nil
}
