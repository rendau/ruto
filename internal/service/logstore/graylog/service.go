package graylog

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

	"github.com/rendau/ruto/internal/errs"
	logstoreModel "github.com/rendau/ruto/internal/service/logstore/model"
)

type Service struct {
	baseURL    string
	apiToken   string
	streamID   string
	baseQuery  string
	httpClient *http.Client
}

func New(baseURL, apiToken, streamID, baseQuery string, timeout time.Duration) *Service {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	// tolerate scheme-less urls like "graylog.logging:9000"
	if !strings.Contains(baseURL, "://") {
		baseURL = "http://" + baseURL
	}

	return &Service{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiToken:  apiToken,
		streamID:  streamID,
		baseQuery: baseQuery,
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

type searchRep struct {
	Messages []struct {
		Message map[string]any `json:"message"`
	} `json:"messages"`
}

func (s *Service) List(ctx context.Context, pars *logstoreModel.ListReq) ([]*logstoreModel.Entry, error) {
	marker := `"access log"`
	if pars.OnlyErrors {
		marker = `"access log error"`
	}

	// The endpoint id is matched both as an extracted field and as full-text, so
	// the query works whether or not the ingest pipeline extracts JSON fields.
	query := `(endpoint_id:` + strconv.Quote(pars.EndpointId) + ` OR ` + strconv.Quote(pars.EndpointId) + `) AND ` + marker
	if s.baseQuery != "" {
		query += ` AND (` + s.baseQuery + `)`
	}

	urlPars := url.Values{
		"query": {query},
		"from":  {pars.Since.UTC().Format("2006-01-02T15:04:05.000Z")},
		"to":    {pars.Until.UTC().Format("2006-01-02T15:04:05.000Z")},
		"limit": {strconv.Itoa(pars.Limit)},
		"sort":  {"timestamp:desc"},
	}
	if s.streamID != "" {
		urlPars.Set("filter", "streams:"+s.streamID)
	}

	repObj := &searchRep{}

	err := s.sendRequest(ctx, "/api/search/universal/absolute", urlPars, repObj)
	if err != nil {
		return nil, fmt.Errorf("sendRequest: %w", err)
	}

	result := make([]*logstoreModel.Entry, 0, len(repObj.Messages))
	for _, message := range repObj.Messages {
		fields := message.Message

		// When the whole slog JSON line lands in the "message" field (no field
		// extraction on the graylog side), parse it to get the structured fields.
		if rawLine, ok := fields["message"].(string); ok && strings.HasPrefix(strings.TrimSpace(rawLine), "{") {
			parsed := map[string]any{}
			if json.Unmarshal([]byte(rawLine), &parsed) == nil {
				for k, v := range fields {
					if _, exists := parsed[k]; !exists {
						parsed[k] = v
					}
				}
				fields = parsed
			}
		}

		if endpointId, ok := fields["endpoint_id"].(string); ok && endpointId != pars.EndpointId {
			continue
		}

		ts := time.Time{}
		if tsStr, ok := fields["timestamp"].(string); ok {
			ts, _ = time.Parse(time.RFC3339Nano, tsStr)
		}

		raw, _ := json.Marshal(message.Message)

		entry := logstoreModel.NewEntryFromFields(ts, fields, string(raw))
		if entry.Message == "" {
			entry.Message, _ = fields["message"].(string)
		}

		result = append(result, entry)
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
	req.SetBasicAuth(s.apiToken, "token")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-By", "ruto-core")

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
