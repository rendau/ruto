package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

const (
	CheckInterval = 5 * time.Minute
)

type Service struct {
	globalCtx   context.Context
	rootSvc     RootServiceI
	appSvc      AppServiceI
	endpointSvc EndpointServiceI

	refreshMu sync.Mutex
	mu        sync.RWMutex
	version   string
	value     []byte
}

func New(
	globalCtx context.Context,
	rootSvc RootServiceI,
	appSvc AppServiceI,
	endpointSvc EndpointServiceI,
) *Service {
	result := &Service{
		globalCtx:   globalCtx,
		rootSvc:     rootSvc,
		appSvc:      appSvc,
		endpointSvc: endpointSvc,
		value:       []byte{},
	}

	go result.worker()

	return result
}

func (s *Service) GetVersion() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

func (s *Service) Get() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

func (s *Service) Refresh() {
	go s.refresh()
}

func (s *Service) worker() {
	// initial refresh
	s.refresh()

	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	for s.globalCtx.Err() == nil {
		select {
		case <-s.globalCtx.Done():
			return
		case <-ticker.C:
			s.refresh()
		}
	}
}

func (s *Service) refresh() {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if s.globalCtx.Err() != nil {
		return
	}

	newValue, err := s.generateFromDb()
	if err != nil {
		slog.Error("snapshot-service: refresh: s.generate", "error", err)
	}

	sum := sha256.Sum256(newValue)
	newVersion := hex.EncodeToString(sum[:])

	s.mu.Lock()
	defer s.mu.Unlock()

	s.version = newVersion
	s.value = newValue
}

func (s *Service) generateFromDb() ([]byte, error) {
	ctx := context.Background()

	// fetch root
	rootObj, err := s.rootSvc.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("rootSvc.Get: %w", err)
	}

	// fetch apps
	apps, _, err := s.appSvc.List(ctx, &appModel.ListReq{})
	if err != nil {
		return nil, fmt.Errorf("appSvc.List: %w", err)
	}
	appByID := lo.SliceToMap(apps, func(app *appModel.App) (string, *appModel.App) {
		app.Endpoints = make([]*endpointModel.Endpoint, 0, 20)
		return app.Id, app
	})

	// fetch endpoints
	endpoints, _, err := s.endpointSvc.List(ctx, &endpointModel.ListReq{})
	if err != nil {
		return nil, fmt.Errorf("endpointSvc.List: %w", err)
	}

	// link endpoints to apps
	lo.ForEach(endpoints, func(ep *endpointModel.Endpoint, _ int) {
		if app, ok := appByID[ep.AppId]; ok {
			app.Endpoints = append(app.Endpoints, ep)
		}
	})

	rootObj.Apps = apps

	// marshal
	result, err := json.Marshal(rootObj)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}

	return result, nil
}
