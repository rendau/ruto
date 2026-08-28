package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/errs"
	prometheusModel "github.com/rendau/ruto/internal/service/prometheus/model"
	localModel "github.com/rendau/ruto/internal/service/prometheus/service/model"
)

type Service struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string, timeout time.Duration) *Service {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	return &Service{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 2 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: 2 * time.Second,
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
				MaxIdleConnsPerHost: 20,
			},
		},
	}
}

func (s *Service) QueryRange(
	ctx context.Context,
	query string,
	start, end time.Time,
	step time.Duration,
) ([]prometheusModel.Series, error) {
	pars := url.Values{
		"query": {query},
		"start": {strconv.FormatInt(start.Unix(), 10)},
		"end":   {strconv.FormatInt(end.Unix(), 10)},
		"step":  {strconv.FormatInt(int64(step.Seconds()), 10)},
	}

	repObj := &localModel.QueryRangeRep{}

	err := s.sendRequest(ctx, "/api/v1/query_range", pars, repObj)
	if err != nil {
		return nil, fmt.Errorf("sendRequest: %w", err)
	}

	if repObj.Status != "success" {
		return nil, fmt.Errorf("prometheus error response: %s: %s: %w", repObj.ErrorType, repObj.Error, errs.ServiceNA)
	}

	return lo.Map(repObj.Data.Result, localModel.DecodeSeries), nil
}

func (s *Service) sendRequest(ctx context.Context, path string, pars url.Values, repObj any) error {
	uri := s.baseURL + path
	if len(pars) > 0 {
		uri += "?" + pars.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return fmt.Errorf("http.NewRequestWithContext: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("httpClient.Do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("bad status code: %d, body: %s: %w", resp.StatusCode, string(body), errs.ServiceNA)
	}

	if err = json.NewDecoder(resp.Body).Decode(repObj); err != nil {
		return fmt.Errorf("json.Decode: %w", err)
	}

	return nil
}
