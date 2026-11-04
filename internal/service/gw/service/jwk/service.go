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

	"encoding/json"

	"github.com/rendau/ruto/internal/constant"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/errs"
)

const (
	ScrapeInterval = 10 * time.Minute
	// ScrapeRetryInterval — укороченный интервал опроса, пока провайдеры заданы, но
	// ни одного ключа нет либо последний опрос какого-то URL-а упал. Без этого
	// авторизация молчит (или живёт на устаревших ключах) до следующего ScrapeInterval.
	ScrapeRetryInterval = 15 * time.Second
)

var (
	instance *Service
)

type Service struct {
	ctx           context.Context
	ctxCancel     context.CancelFunc
	providerStore atomic.Pointer[[]rootModel.RootJwt]
	itemStore     atomic.Pointer[keySet]
	loadMu        sync.Mutex
	urlItems      map[string][]*Item // под loadMu: ключи последнего удачного опроса по каждому URL
	loadFailed    atomic.Bool
	trigger       chan struct{}
	httpClient    *http.Client
}

// keySet — ключи всех провайдеров: общий индекс по kid для методов без
// провайдера и индекс по имени провайдера для методов с ним.
type keySet struct {
	all        map[string]*Item
	byProvider map[string]map[string]*Item
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
				// Опрос раз в ScrapeInterval: сервер закроет простаивающее соединение
				// раньше, чем оно понадобится снова, поэтому держать его незачем.
				DisableKeepAlives: true,
			},
		},
	}

	go instance.worker()
}

func Ins() *Service {
	return instance
}

func (s *Service) SetProviders(providers []rootModel.RootJwt) {
	providersCopy := make([]rootModel.RootJwt, len(providers))
	copy(providersCopy, providers)
	s.providerStore.Store(&providersCopy)
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

// GetPublicKey returns the key kid of the named provider, or of any provider
// when provider is empty.
func (s *Service) GetPublicKey(provider, kid string) (*rsa.PublicKey, string) {
	set := s.getItems()
	items := set.all
	if provider != "" {
		items = set.byProvider[provider]
	}
	if item := items[kid]; item != nil {
		return &item.RSAPublicKey, item.Alg
	}
	return nil, ""
}

func Stop() {
	instance.ctxCancel()
}

func (s *Service) getProviders() []rootModel.RootJwt {
	providersPtr := s.providerStore.Load()
	if providersPtr == nil {
		return nil
	}
	return *providersPtr
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

// needFastRetry — true, когда опрашивать нужно, но ключей ещё нет (типично для
// первого старта при недоступном JWK-сервере) либо последний опрос какого-то
// URL-а упал.
func (s *Service) needFastRetry() bool {
	return len(s.getProviders()) > 0 && (len(s.getItems().all) == 0 || s.loadFailed.Load())
}

func (s *Service) setItems(set *keySet) {
	s.itemStore.Store(set)
}

func (s *Service) getItems() *keySet {
	if set := s.itemStore.Load(); set != nil {
		return set
	}
	return &keySet{}
}

func (s *Service) load() {
	if s.ctx.Err() != nil {
		return
	}

	s.loadMu.Lock()
	defer s.loadMu.Unlock()

	ctx := s.ctx

	providers := s.getProviders()
	if len(providers) == 0 {
		s.urlItems = nil
		s.loadFailed.Store(false)
		s.setItems(nil)
		return
	}

	var (
		failed   bool
		urlItems = make(map[string][]*Item, len(providers))
		result   = &keySet{
			all:        make(map[string]*Item, len(providers)*2),
			byProvider: make(map[string]map[string]*Item, len(providers)),
		}
	)

	for _, provider := range providers {
		if ctx.Err() != nil {
			return
		}

		// Один URL у нескольких провайдеров опрашивается один раз.
		items, loaded := urlItems[provider.JwkUrl]
		if !loaded {
			var ok bool
			items, ok = s.loadItems(ctx, provider.JwkUrl)
			if ctx.Err() != nil {
				return
			}
			failed = failed || !ok
			urlItems[provider.JwkUrl] = items
		}

		for _, item := range items {
			result.all[item.Kid] = item
		}
		if provider.Name != "" {
			byKid := result.byProvider[provider.Name]
			if byKid == nil {
				byKid = make(map[string]*Item, len(items))
				result.byProvider[provider.Name] = byKid
			}
			for _, item := range items {
				byKid[item.Kid] = item
			}
		}
	}

	s.urlItems = urlItems
	s.loadFailed.Store(failed)
	s.setItems(result)
}

// loadItems returns the usable keys of uri. On a failed poll ok is false and
// the keys of the last successful poll are returned: a one-off failure must not
// revoke working keys.
func (s *Service) loadItems(ctx context.Context, uri string) (_ []*Item, ok bool) {
	itemSet, err := s.loadForUri(ctx, uri)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error(
				"jwk-scrapper: loadForUri failed",
				"error", fmt.Errorf("loadForUri: %w", err),
				"uri", uri,
			)
		}
		return s.urlItems[uri], false
	}

	items := make([]*Item, 0, len(itemSet.Keys))
	for _, item := range itemSet.Keys {
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
		items = append(items, item)
	}

	return items, true
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
