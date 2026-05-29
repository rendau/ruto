package swagger

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Service struct {
	httpClient *http.Client
}

type Endpoint struct {
	Method string
	Path   string
}

func New(timeout time.Duration) *Service {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Service{
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (s *Service) LoadEndpoints(ctx context.Context, swaggerURL string) ([]Endpoint, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, swaggerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequestWithContext: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.Client.Do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("bad status code: %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("io.ReadAll: %w", err)
	}

	raw, err := parseDocumentJSON(body)
	if err == nil {
		return parseEndpoints(raw), nil
	}

	raw, yamlErr := parseDocumentYAML(body)
	if yamlErr == nil {
		return parseEndpoints(raw), nil
	}

	return nil, fmt.Errorf("unable to parse swagger document as JSON or YAML: json: %v; yaml: %v", err, yamlErr)
}
