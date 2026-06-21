package jwk

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goccy/go-json"

	"github.com/rendau/ruto/internal/constant"
	"github.com/rendau/ruto/internal/errs"
)

const (
	ScrapeInterval = 10 * time.Minute
	// ScrapeRetryInterval — укороченный интервал опроса, пока URL-ы заданы, но ни
	// одного ключа загрузить не удалось (например, JWK-сервер недоступен на старте).
	// Без этого вся авторизация молчит до следующего ScrapeInterval.
	ScrapeRetryInterval = 15 * time.Second
)

var (
	instance *Service
)

type Service struct {
	ctx        context.Context
	ctxCancel  context.CancelFunc
	urlStore   atomic.Pointer[[]string]
	itemStore  atomic.Pointer[map[string]*Item]
	loadMu     sync.Mutex
	trigger    chan struct{}
	httpClient *http.Client
}

func init() {
	ctx, ctxCancel := context.WithCancel(context.Background())

	instance = &Service{
		ctx:       ctx,
		ctxCancel: ctxCancel,
		trigger:   make(chan struct{}, 1),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 2 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: 2 * time.Second,
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
				MaxIdleConnsPerHost: 2,
			},
		},
	}

	go instance.worker()
}

func Ins() *Service {
	return instance
}

func (s *Service) SetUrls(urls []string) {
	urlsCopy := make([]string, len(urls))
	copy(urlsCopy, urls)
	s.urlStore.Store(&urlsCopy)
	s.signal()
}

// signal будит worker, чтобы он немедленно перезагрузил ключи и пересчитал
// интервал опроса. Неблокирующий: если сигнал уже в очереди — пропускаем.
func (s *Service) signal() {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

func (s *Service) GetPublicKey(kid string) (*rsa.PublicKey, string) {
	if item := s.getItems()[kid]; item != nil {
		return &item.RSAPublicKey, item.Alg
	}
	return nil, ""
}

func Stop() {
	instance.ctxCancel()
}

func (s *Service) getUrls() []string {
	urlsPtr := s.urlStore.Load()
	if urlsPtr == nil || *urlsPtr == nil {
		return []string{}
	}
	return *urlsPtr
}

func (s *Service) worker() {
	for s.ctx.Err() == nil {
		s.load()

		interval := ScrapeInterval
		if s.needFastRetry() {
			interval = ScrapeRetryInterval
		}

		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
		case <-s.trigger:
			timer.Stop()
		case <-s.ctx.Done():
			timer.Stop()
			return
		}
	}
}

// needFastRetry — true, когда опрашивать нужно, но ни одного ключа ещё нет
// (URL-ы заданы, карта ключей пуста). Типично для первого старта при недоступном
// JWK-сервере.
func (s *Service) needFastRetry() bool {
	return len(s.getUrls()) > 0 && len(s.getItems()) == 0
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
	if s.ctx.Err() != nil {
		return
	}

	s.loadMu.Lock()
	defer s.loadMu.Unlock()

	ctx := s.ctx

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

		if ctx.Err() != nil {
			return
		}

		for _, item = range itemSet.Keys {
			if !constant.IsSupportedJWTAlgorithm(item.Alg) {
				continue
			}
			item.RSAPublicKey, err = toRSAPublicKey(item)
			if err != nil {
				slog.Error(
					"jwk-scrapper: invalid key",
					"error", fmt.Errorf("ToRSAPublicKey: %w", err),
					"kid", item.Kid,
					"uri", uri,
				)
				continue
			}
			result[item.Kid] = item
		}
	}

	// slog.Debug("JWK items loaded", "key_count", len(result))

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
