package gateway

import (
	"context"
	"time"

	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
)

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
}

type CacheI interface {
	ListKeys() ([]string, error)
	SetJsonObj(key string, value any, ttl time.Duration) error
	GetJsonObj(key string, dst any) (bool, error)
}

type GatewaysI interface {
	Register(gatewayID string, notify func()) int64
	Unregister(id int64)
}
