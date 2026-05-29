package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

const swaggerDiscoveryBackfillWorkers = 2

type swaggerDiscoveryResult struct {
	updated bool
	failed  bool
}

func (u *Usecase) BackfillSwaggerURLs(ctx context.Context) error {
	const pageSize = int64(200)
	startedAt := time.Now()

	var (
		page          int64
		processed     int64
		updated       int64
		discoveryFail int64
	)

	for {
		items, _, err := u.svc.List(ctx, &appModel.ListReq{
			ListParams: commonModel.ListParams{
				Page:     page,
				PageSize: pageSize,
			},
		})
		if err != nil {
			return fmt.Errorf("svc.List: %w", err)
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

		batchUpdated, batchFailed := u.runSwaggerDiscoveryBatch(ctx, candidates)
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

func (u *Usecase) runSwaggerDiscoveryBatch(ctx context.Context, items []*appModel.App) (updated, failed int64) {
	if len(items) == 0 {
		return 0, 0
	}

	workers := swaggerDiscoveryBackfillWorkers
	if workers > len(items) {
		workers = len(items)
	}
	if workers < 1 {
		workers = 1
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(workers)

	var updatedCount atomic.Int64
	var failedCount atomic.Int64

	for _, item := range items {
		group.Go(func() error {
			if groupCtx.Err() != nil {
				return nil
			}

			result := u.discoverAndUpdateAppSwaggerURL(groupCtx, item)
			if result.updated {
				updatedCount.Add(1)
			}
			if result.failed {
				failedCount.Add(1)
			}
			return nil
		})
	}

	_ = group.Wait()

	return updatedCount.Load(), failedCount.Load()
}

func (u *Usecase) discoverAndUpdateAppSwaggerURL(ctx context.Context, item *appModel.App) swaggerDiscoveryResult {
	swaggerURL, discoverErr := u.GetSwaggerURLByBackendURL(ctx, item.Backend.Url)
	if discoverErr != nil {
		return swaggerDiscoveryResult{failed: true}
	}
	if strings.TrimSpace(swaggerURL) == "" {
		return swaggerDiscoveryResult{}
	}

	item.Backend.SwaggerUrl = swaggerURL
	if updateErr := u.Update(ctx, item.Id, item); updateErr != nil {
		return swaggerDiscoveryResult{failed: true}
	}

	return swaggerDiscoveryResult{updated: true}
}
