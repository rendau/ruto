// Package cloudflare keeps the list of Cloudflare's networks. A request that
// came from one of them carries the client address in Cf-Connecting-Ip, which
// Cloudflare always writes itself, so the client cannot forge it.
//
// The built-in list serves from the start; Run replaces it with the one
// Cloudflare publishes and refreshes it. A failed or implausible answer keeps
// the list in use.
package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/infra/httpx"
)

const (
	requestTimeout  = 10 * time.Second
	maxResponseSize = 1 << 20
	// a failed refresh is retried sooner than the regular interval
	retryInterval = 5 * time.Minute

	// A network broader than these would let anybody claim any address through
	// Cf-Connecting-Ip; Cloudflare's broadest are /13 and /29.
	minPrefixBitsV4 = 8
	minPrefixBitsV6 = 20
)

// builtin is the list Cloudflare published on 2026-11-25.
var builtin = []string{
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
	"2400:cb00::/32",
	"2606:4700::/32",
	"2803:f800::/32",
	"2405:b500::/32",
	"2405:8100::/32",
	"2a06:98c0::/29",
	"2c0f:f248::/32",
}

type Service struct {
	url        string
	interval   time.Duration
	httpClient *http.Client
	networks   atomic.Pointer[[]netip.Prefix]
	etag       string // only the Run goroutine touches it
}

// New takes the address of Cloudflare's list and how often to fetch it; a zero
// interval keeps the built-in list.
func New(url string, interval time.Duration) *Service {
	service := &Service{
		url:        url,
		interval:   interval,
		httpClient: httpx.New(httpx.Config{Timeout: requestTimeout, DisableKeepAlives: true, VerifyTLS: true}),
	}

	networks, err := parseNetworks(builtin)
	if err != nil {
		panic(fmt.Sprintf("cloudflare: built-in list: %v", err))
	}
	service.networks.Store(&networks)

	return service
}

func (s *Service) Contains(ip netip.Addr) bool {
	ip = ip.Unmap()
	return lo.ContainsBy(*s.networks.Load(), func(network netip.Prefix) bool { return network.Contains(ip) })
}

func (s *Service) Networks() []netip.Prefix {
	return *s.networks.Load()
}

func (s *Service) Run(ctx context.Context) {
	if s.interval <= 0 || s.url == "" {
		slog.Info("cloudflare: refresh is off, the built-in list of networks is used")
		return
	}

	for {
		wait := s.interval
		if err := s.refresh(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("cloudflare: refresh of networks failed, the list in use is kept", "url", s.url, "error", err)
			wait = min(wait, retryInterval)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

type ipsRep struct {
	Success bool `json:"success"`
	Result  struct {
		IPv4 []string `json:"ipv4_cidrs"`
		IPv6 []string `json:"ipv6_cidrs"`
		Etag string   `json:"etag"`
	} `json:"result"`
}

func (s *Service) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return fmt.Errorf("http.NewRequestWithContext: %w", err)
	}

	rep, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("httpClient.Do: %w", err)
	}
	defer rep.Body.Close()

	if rep.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %d", rep.StatusCode)
	}

	var body ipsRep
	if err = json.NewDecoder(io.LimitReader(rep.Body, maxResponseSize)).Decode(&body); err != nil {
		return fmt.Errorf("json.Decode: %w", err)
	}
	if !body.Success {
		return errors.New("answer is not successful")
	}
	if len(body.Result.IPv4) == 0 {
		return errors.New("answer has no IPv4 networks")
	}
	if body.Result.Etag != "" && body.Result.Etag == s.etag {
		return nil
	}

	networks, err := parseNetworks(append(body.Result.IPv4, body.Result.IPv6...))
	if err != nil {
		return err
	}

	s.networks.Store(&networks)
	s.etag = body.Result.Etag
	slog.Info("cloudflare: networks updated", "count", len(networks), "etag", body.Result.Etag)

	return nil
}

func parseNetworks(items []string) ([]netip.Prefix, error) {
	result := make([]netip.Prefix, 0, len(items))
	for _, raw := range items {
		network, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("netip.ParsePrefix(%q): %w", raw, err)
		}
		minBits := minPrefixBitsV6
		if network.Addr().Is4() {
			minBits = minPrefixBitsV4
		}
		if network.Bits() < minBits {
			return nil, fmt.Errorf("network %s is too broad", raw)
		}
		result = append(result, network.Masked())
	}
	return result, nil
}
