package jwk

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rendau/ruto/internal/errs"
)

const (
	ScrapeInterval = 10 * time.Minute
)

type Service struct {
	globalCtx  context.Context
	urlStore   atomic.Pointer[[]string]
	itemStore  atomic.Pointer[map[string]*Item]
	loadMu     sync.Mutex
	httpClient *http.Client
}

func New(globalCtx context.Context) *Service {
	return &Service{
		globalCtx: globalCtx,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 2 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: 2 * time.Second,
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
				MaxIdleConnsPerHost: 10,
			},
		},
	}
}

func (s *Service) Start() {
	go s.worker()
}

func (s *Service) SetUrls(urls []string) {
	urlsCopy := make([]string, len(urls))
	copy(urlsCopy, urls)
	s.urlStore.Store(&urlsCopy)
	go s.load()
}

func (s *Service) Get(kid string) *Item {
	items := s.getItems()
	return items[kid]
}

func (s *Service) getUrls() []string {
	urlsPtr := s.urlStore.Load()
	if urlsPtr == nil || *urlsPtr == nil {
		return []string{}
	}
	return *urlsPtr
}

func (s *Service) worker() {
	// ticker
	ticker := time.NewTicker(ScrapeInterval)
	defer ticker.Stop()

	for s.globalCtx.Err() == nil {
		s.load()

		select {
		case <-ticker.C:
		case <-s.globalCtx.Done():
		}
	}
}

func (s *Service) setItems(items map[string]*Item) {
	s.itemStore.Store(&items)
}

func (s *Service) getItems() map[string]*Item {
	itemsPtr := s.itemStore.Load()
	if itemsPtr == nil || *itemsPtr == nil {
		return map[string]*Item{}
	}
	return *itemsPtr
}

func (s *Service) load() {
	s.loadMu.Lock()
	defer s.loadMu.Unlock()

	ctx := s.globalCtx

	urls := s.getUrls()
	if len(urls) == 0 {
		return
	}

	var (
		err     error
		itemSet *ItemSet
		item    *Item
		result  = make(map[string]*Item, len(urls)*2)
	)

	for _, uri := range urls {
		if ctx.Err() != nil {
			return
		}

		itemSet, err = s.loadForUri(ctx, uri)
		if err != nil {
			if ctx.Err() == nil {
				slog.Error(
					"jwk-scrapper: loadForUri failed",
					"error", fmt.Errorf("loadForUri: %w", err),
					"uri", uri,
				)
			}
			continue
		}

		for _, item = range itemSet.Keys {
			result[item.Kid] = item
		}
	}

	s.setItems(result)
}

func (s *Service) loadForUri(ctx context.Context, uri string) (*ItemSet, error) {
	repObj := new(ItemSet)
	_, err := s.sendRequest(ctx, http.MethodGet, uri, repObj)
	if err != nil {
		return nil, fmt.Errorf("sendRequest: %w", err)
	}

	return repObj, nil
}

func (s *Service) sendRequest(ctx context.Context, method string, uri string, repObj any) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, uri, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.Client.Do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	repBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("io.ReadAll: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bad status code: %d, body: %s, %w", resp.StatusCode, string(repBody), errs.ServiceNA)
	}

	if repObj != nil {
		err = json.Unmarshal(repBody, repObj)
		if err != nil {
			return repBody, fmt.Errorf("json.Unmarshal: %w, uri: %s, body: %s", err, uri, string(repBody))
		}
	}

	return repBody, nil
}
