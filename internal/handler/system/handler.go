// Package system is the transport of core's system port: the routes for
// pulse's ruto integration and core's part of the pulse manifest (the gateways
// endpoint, gauges, change request counts).
package system

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/samber/lo"

	changeRequestModel "github.com/rendau/ruto/internal/domain/changerequest/model"
	"github.com/rendau/ruto/internal/infra/pulsekit"
	usecaseGateway "github.com/rendau/ruto/internal/usecase/gateway"
	usecaseSystem "github.com/rendau/ruto/internal/usecase/system"
)

// ChangeRequestEntity is the change request in the manifest's domain section.
const ChangeRequestEntity = "запрос на изменение"

type Handler struct {
	uc UsecaseI
}

func New(uc UsecaseI) *Handler {
	return &Handler{uc: uc}
}

// Register adds the routes for pulse: /routes/version is cheap and polled,
// /routes is read again only when the version changes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /routes/version", func(w http.ResponseWriter, r *http.Request) {
		version, err := h.uc.RoutesVersion(r.Context())
		if err != nil {
			writeError(w, "routes version", err)
			return
		}
		writeJson(w, http.StatusOK, &RoutesVersionRep{Version: version})
	})

	mux.HandleFunc("GET /routes", func(w http.ResponseWriter, r *http.Request) {
		routes, err := h.uc.Routes(r.Context())
		if err != nil {
			writeError(w, "routes", err)
			return
		}
		writeJson(w, http.StatusOK, EncodeRoutes(routes))
	})
}

// RegisterPulse declares core's gauges, the change request report and the
// gateways endpoint in the pulse manifest.
func (h *Handler) RegisterPulse(kit *pulsekit.Kit) {
	kit.Gauge("gateways_online", "Шлюзов на связи", "count", func(ctx context.Context) (float64, string, error) {
		return h.countGateways(ctx, func(gw *usecaseSystem.Gateway) bool { return gw.Status == usecaseGateway.StatusOnline }, true)
	})
	kit.Gauge("gateways_lagging", "Шлюзов на старой версии дольше 2 мин", "count", func(ctx context.Context) (float64, string, error) {
		return h.countGateways(ctx, func(gw *usecaseSystem.Gateway) bool { return gw.Lagging }, false)
	})
	kit.Gauge("gateways_silent", "Шлюзов на связи, но без отчётов дольше 3 мин", "count", func(ctx context.Context) (float64, string, error) {
		return h.countGateways(ctx, func(gw *usecaseSystem.Gateway) bool { return gw.Silent }, false)
	})
	kit.Gauge("unpublished_changes", "Есть неопубликованные правки (1 — да)", "count", func(ctx context.Context) (float64, string, error) {
		has, err := h.uc.HasUnpublishedChanges(ctx)
		return lo.Ternary(has, 1.0, 0.0), "ok", err
	})

	kit.Activity(ChangeRequestEntity, 0, func(ctx context.Context, stuckAfter map[string]time.Duration) (pulsekit.EntityCounts, error) {
		stats, err := h.uc.PendingChangeRequests(ctx, stuckAfter[changeRequestModel.StatusPending])
		if err != nil {
			return pulsekit.EntityCounts{}, err
		}
		pending := pulsekit.StatusCount{Count: stats.Count, Stuck: stats.Stuck}
		if stats.OldestAt != nil {
			pending.Oldest = time.Since(*stats.OldestAt)
		}
		return pulsekit.EntityCounts{Statuses: map[string]pulsekit.StatusCount{changeRequestModel.StatusPending: pending}}, nil
	})

	pulsekit.Handle(kit, pulsekit.Endpoint{
		Id:    "gateways",
		Title: "Шлюзы и их версия конфигурации",
		Description: "Вызывай, когда спрашивают, применились ли изменения в ruto, почему новый или изменённый маршрут " +
			"не работает, все ли шлюзы на связи. Вернёт опубликованную версию, есть ли неопубликованные правки " +
			"(сохранены в админке, но не опубликованы — до шлюзов не дошли) и по каждому шлюзу: на связи ли, " +
			"его версию, на опубликованной ли он версии, когда применил конфигурацию, последний сбой. " +
			"lagging или silent у шлюза — проблема. Не для ошибок сервисов за шлюзом.",
		Path:     "/diag/gateways",
		Timeout:  3 * time.Second,
		RowsPath: "gateways",
	}, func(ctx context.Context, _ map[string]string) (*GatewaysRep, error) {
		gateways, err := h.uc.Gateways(ctx)
		if err != nil {
			return nil, err
		}
		has, err := h.uc.HasUnpublishedChanges(ctx)
		if err != nil {
			return nil, err
		}
		return EncodeGateways(gateways, has), nil
	})
}

// countGateways — how many gateways match; any of them is a problem (degraded)
// unless degradedAtZero, where none is.
func (h *Handler) countGateways(ctx context.Context, match func(gw *usecaseSystem.Gateway) bool, degradedAtZero bool) (float64, string, error) {
	gateways, err := h.uc.Gateways(ctx)
	if err != nil {
		return 0, "", err
	}
	n := lo.CountBy(gateways.Gateways, match)
	degraded := lo.Ternary(degradedAtZero, n == 0, n > 0)
	return float64(n), lo.Ternary(degraded, "degraded", "ok"), nil
}

func writeError(w http.ResponseWriter, what string, err error) {
	slog.Error("system: "+what+" failed", "error", err)
	writeJson(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

func writeJson(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("system: write response failed", "error", err)
	}
}
