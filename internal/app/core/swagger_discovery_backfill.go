package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

const swaggerDiscoveryWorkers = 2

type swaggerDiscoveryResult struct {
	updated bool
	failed  bool
}

func discoverAndUpdateAppSwaggerURL(
	ctx context.Context,
	appUsecase interface {
		GetSwaggerURLByBackendURL(context.Context, string) (string, error)
		Update(context.Context, string, *appModel.App) error
	},
	item *appModel.App,
) swaggerDiscoveryResult {
	swaggerURL, discoverErr := appUsecase.GetSwaggerURLByBackendURL(ctx, item.Backend.Url)
	if discoverErr != nil {
		return swaggerDiscoveryResult{failed: true}
	}
	if strings.TrimSpace(swaggerURL) == "" {
		return swaggerDiscoveryResult{}
	}

	item.Backend.SwaggerUrl = swaggerURL
	if updateErr := appUsecase.Update(ctx, item.Id, item); updateErr != nil {
		return swaggerDiscoveryResult{failed: true}
	}

	return swaggerDiscoveryResult{updated: true}
}

func runSwaggerDiscoveryBatch(
	ctx context.Context,
	appUsecase interface {
		GetSwaggerURLByBackendURL(context.Context, string) (string, error)
		Update(context.Context, string, *appModel.App) error
	},
	items []*appModel.App,
) (updated, failed int64) {
	if len(items) == 0 {
		return 0, 0
	}

	workers := swaggerDiscoveryWorkers
	if workers > len(items) {
		workers = len(items)
	}
	if workers < 1 {
		workers = 1
	}

	jobs := make(chan *appModel.App)
	results := make(chan swaggerDiscoveryResult, len(items))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				results <- discoverAndUpdateAppSwaggerURL(ctx, appUsecase, item)
			}
		}()
	}

dispatchLoop:
	for _, item := range items {
		select {
		case <-ctx.Done():
			break dispatchLoop
		case jobs <- item:
		}
	}
	close(jobs)
	wg.Wait()
	close(results)

	for result := range results {
		if result.updated {
			updated++
		}
		if result.failed {
			failed++
		}
	}

	return updated, failed
}

func backfillAppSwaggerURLs(
	ctx context.Context,
	appSvc interface {
		List(context.Context, *appModel.ListReq) ([]*appModel.App, int64, error)
	},
	appUsecase interface {
		GetSwaggerURLByBackendURL(context.Context, string) (string, error)
		Update(context.Context, string, *appModel.App) error
	},
) error {
	const pageSize = int64(200)
	startedAt := time.Now()

	var (
		page          int64
		processed     int64
		updated       int64
		discoveryFail int64
	)

	for {
		items, _, err := appSvc.List(ctx, &appModel.ListReq{
			ListParams: commonModel.ListParams{
				Page:     page,
				PageSize: pageSize,
			},
		})
		if err != nil {
			return fmt.Errorf("appSvc.List: %w", err)
		}

		if len(items) == 0 {
			break
		}

		candidates := make([]*appModel.App, 0, len(items))
		for _, item := range items {
			processed++
			if item == nil {
				continue
			}

			if strings.TrimSpace(item.Backend.SwaggerUrl) != "" || strings.TrimSpace(item.Backend.Url) == "" {
				continue
			}
			candidates = append(candidates, item)
		}
		batchUpdated, batchFailed := runSwaggerDiscoveryBatch(ctx, appUsecase, candidates)
		updated += batchUpdated
		discoveryFail += batchFailed

		if int64(len(items)) < pageSize {
			break
		}
		page++
	}

	durStr := time.Since(startedAt).String()

	slog.Info(
		"app swagger discovery on start finished "+durStr,
		"apps_processed", processed,
		"apps_updated", updated,
		"errors", discoveryFail,
		"duration", durStr,
	)

	return nil
}
