package loki

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rendau/ruto/internal/errs"
	logstoreModel "github.com/rendau/ruto/internal/service/logstore/model"
)

type Service struct {
	baseURL    string
	selector   string
	orgID      string
	httpClient *http.Client
}

func New(baseURL, selector, orgID string, timeout time.Duration) *Service {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	// tolerate scheme-less urls like "loki.monitoring:3100"
	if !strings.Contains(baseURL, "://") {
		baseURL = "http://" + baseURL
	}

	return &Service{
		baseURL:  strings.TrimRight(baseURL, "/"),
		selector: selector,
		orgID:    orgID,
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

type queryRangeRep struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Stream map[string]string `json:"stream"`
			Values [][2]string       `json:"values"` // [ts_ns, line]
		} `json:"result"`
	} `json:"data"`
}

func (s *Service) List(ctx context.Context, pars *logstoreModel.ListReq) ([]*logstoreModel.Entry, error) {
	lineFilter := ` |= "access log"`
	if pars.OnlyErrors {
		lineFilter = ` |= "access log error"`
	}
	query := s.selector + lineFilter + ` | json | endpoint_id=` + strconv.Quote(pars.EndpointId)

	urlPars := url.Values{
		"query":     {query},
		"start":     {strconv.FormatInt(pars.Since.UnixNano(), 10)},
		"end":       {strconv.FormatInt(pars.Until.UnixNano(), 10)},
		"limit":     {strconv.Itoa(pars.Limit)},
		"direction": {"backward"},
	}

	repObj := &queryRangeRep{}

	err := s.sendRequest(ctx, "/loki/api/v1/query_range", urlPars, repObj)
	if err != nil {
		return nil, fmt.Errorf("sendRequest: %w", err)
	}

	result := make([]*logstoreModel.Entry, 0, pars.Limit)
	for _, stream := range repObj.Data.Result {
		for _, value := range stream.Values {
			tsNs, parseErr := strconv.ParseInt(value[0], 10, 64)
			if parseErr != nil {
				continue
			}

			fields := map[string]any{}
			_ = json.Unmarshal([]byte(value[1]), &fields)

			result = append(result, logstoreModel.NewEntryFromFields(time.Unix(0, tsNs), fields, value[1]))
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Ts.After(result[j].Ts) })
	if pars.Limit > 0 && len(result) > pars.Limit {
		result = result[:pars.Limit]
	}

	return result, nil
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
	if s.orgID != "" {
		req.Header.Set("X-Scope-OrgID", s.orgID)
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
