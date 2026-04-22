package jwk

import (
	"context"
	"log/slog"
	"time"
)

const (
	ScrapeInterval = 10 * time.Minute
)

type Service struct {
	globalCtx context.Context
	urls      []string
}

func New(globalCtx context.Context) *Service {
	return &Service{
		globalCtx: globalCtx,
	}
}

func (s *Service) Start() {
	go s.worker()
}

func (s *Service) SetUrls(urls []string) {
	s.urls = urls
}

func (s *Service) worker() {
	// ticker
	var err error
	ticker := time.NewTicker(ScrapeInterval)
	defer ticker.Stop()

	for s.globalCtx.Err() == nil {
		err = s.run()
		if err != nil {
			slog.Error("jwk-scrapper: run failed", "error", err)
		}

		select {
		case <-ticker.C:
		case <-s.globalCtx.Done():
		}
	}
}

func (s *Service) run() error {
	return nil
}
