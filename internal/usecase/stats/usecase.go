package stats

import (
	"context"
	"fmt"
	"sort"
	"time"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	usrModel "github.com/rendau/ruto/internal/domain/usr/model"
)

type Usecase struct {
	rootSvc     RootServiceI
	appSvc      AppServiceI
	endpointSvc EndpointServiceI
	usrSvc      UsrServiceI
	startedAt   time.Time
}

func New(rootSvc RootServiceI, appSvc AppServiceI, endpointSvc EndpointServiceI, usrSvc UsrServiceI, startedAt time.Time) *Usecase {
	return &Usecase{
		rootSvc:     rootSvc,
		appSvc:      appSvc,
		endpointSvc: endpointSvc,
		usrSvc:      usrSvc,
		startedAt:   startedAt,
	}
}

func (u *Usecase) Get(ctx context.Context) (*Stats, error) {
	rootObj, err := u.rootSvc.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("rootSvc.Get: %w", err)
	}

	apps, _, err := u.appSvc.List(ctx, &appModel.ListReq{})
	if err != nil {
		return nil, fmt.Errorf("appSvc.List: %w", err)
	}

	endpoints, _, err := u.endpointSvc.List(ctx, &endpointModel.ListReq{})
	if err != nil {
		return nil, fmt.Errorf("endpointSvc.List: %w", err)
	}

	users, _, err := u.usrSvc.List(ctx, &usrModel.ListReq{})
	if err != nil {
		return nil, fmt.Errorf("usrSvc.List: %w", err)
	}

	stats := &Stats{
		AppsTotal:         int64(len(apps)),
		EndpointsTotal:    int64(len(endpoints)),
		UsersTotal:        int64(len(users)),
		RootJWTProviders:  int64(len(rootObj.Jwt)),
		RootAuthEnabled:   rootObj.Auth.Enabled,
		RootCorsEnabled:   rootObj.Cors.Enabled,
		CoreUptimeSeconds: int64(time.Since(u.startedAt).Seconds()),
	}

	for _, app := range apps {
		if app.Active {
			stats.AppsActive++
		}
	}
	stats.AppsInactive = stats.AppsTotal - stats.AppsActive

	methodMap := make(map[string]*MethodStats, 8)
	for _, endpoint := range endpoints {
		if endpoint.Active {
			stats.EndpointsActive++
		}

		method := endpoint.Http.Method
		if method == "" {
			method = "*"
		}

		methodStats, ok := methodMap[method]
		if !ok {
			methodStats = &MethodStats{Method: method}
			methodMap[method] = methodStats
		}
		methodStats.Total++
		if endpoint.Active {
			methodStats.Active++
		}
	}
	stats.EndpointsInactive = stats.EndpointsTotal - stats.EndpointsActive

	for _, user := range users {
		if user.Active {
			stats.UsersActive++
		}
		if user.IsAdmin {
			stats.UsersAdmin++
		}
	}

	stats.Methods = make([]MethodStats, 0, len(methodMap))
	for _, methodStats := range methodMap {
		stats.Methods = append(stats.Methods, *methodStats)
	}
	sort.Slice(stats.Methods, func(i, j int) bool {
		if stats.Methods[i].Total == stats.Methods[j].Total {
			return stats.Methods[i].Method < stats.Methods[j].Method
		}
		return stats.Methods[i].Total > stats.Methods[j].Total
	})

	return stats, nil
}
