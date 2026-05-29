package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

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

		for _, item := range items {
			processed++
			if item == nil {
				continue
			}

			if strings.TrimSpace(item.Backend.SwaggerUrl) != "" || strings.TrimSpace(item.Backend.Url) == "" {
				continue
			}

			swaggerURL, discoverErr := appUsecase.GetSwaggerURLByBackendURL(ctx, item.Backend.Url)
			if discoverErr != nil {
				discoveryFail++
				continue
			}
			if strings.TrimSpace(swaggerURL) == "" {
				continue
			}

			item.Backend.SwaggerUrl = swaggerURL
			if updateErr := appUsecase.Update(ctx, item.Id, item); updateErr != nil {
				discoveryFail++
				continue
			}

			updated++
		}

		if int64(len(items)) < pageSize {
			break
		}
		page++
	}

	slog.Info(
		"app swagger discovery on start finished",
		"apps_processed", processed,
		"apps_updated", updated,
		"errors", discoveryFail,
		"duration", time.Since(startedAt).String(),
	)

	return nil
}
