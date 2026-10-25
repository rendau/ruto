package model

import (
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	loggingModel "github.com/rendau/ruto/internal/domain/logging/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

type Type string

const (
	TypeHTTP Type = "http"
	TypeGRPC Type = "grpc"
)

type Endpoint struct {
	Id                 string               `json:"id"`
	AppId              string               `json:"app_id"`
	Active             bool                 `json:"active"`
	ExcludeFromMetrics bool                 `json:"exclude_from_metrics"`
	TrackPaths         bool                 `json:"track_paths"`
	AllowTrailingSlash bool                 `json:"allow_trailing_slash"` // "users/" also reaches an exact path "users"; only admins change it
	Type               Type                 `json:"type"`
	Http               Http                 `json:"http"`
	Grpc               Grpc                 `json:"grpc"`
	Backend            Backend              `json:"backend"`
	Auth               authModel.Auth       `json:"auth"`
	Logging            loggingModel.Logging `json:"logging"`
	Variables          varsModel.Vars       `json:"variables"`
	Transform          Transform            `json:"transform"`
	Cache              Cache                `json:"cache"`
}

// Cache configures response caching of an http endpoint. Server keeps
// responses in the gateway memory and answers repeated GET requests itself;
// Browser only sets Cache-Control so the client keeps the response.
type Cache struct {
	Server  CacheServer  `json:"server"`
	Browser CacheBrowser `json:"browser"`
}

type CacheServer struct {
	Enabled bool `json:"enabled"`
	Ttl     int  `json:"ttl"` // seconds
	// Shared keeps one cached response for all callers: the credentials are
	// left out of the cache key even when auth is enabled. Only for responses
	// that do not depend on who asks.
	Shared bool `json:"shared"`
	// VaryHeaders are request headers whose values keep separate cached
	// responses, on top of path, query, Accept-Encoding and the caller's
	// credentials when auth is enabled and the cache is not shared.
	VaryHeaders []string `json:"vary_headers"`
	MaxBodySize int      `json:"max_body_size"` // bytes, 0 => DefaultCacheMaxBodySize
}

type CacheBrowser struct {
	Enabled bool   `json:"enabled"`
	Scope   string `json:"scope"`   // "private" | "public" | "no-store"
	MaxAge  int    `json:"max_age"` // seconds, ignored for "no-store"
	// Override replaces the Cache-Control the backend sent; otherwise it is set
	// only when the backend sent none.
	Override bool `json:"override"`
}

// Transform holds optional scripts (JavaScript, evaluated by the gateway) that
// reshape a request before it is proxied to the backend, and/or the response
// before it is returned to the client.
type Transform struct {
	Request  string `json:"request"`
	Response string `json:"response"`
	// MaxWorkers caps concurrent goja runtimes per script (memory bound).
	// 0 falls back to the Root-level default, then the engine default.
	MaxWorkers int `json:"max_workers"`
}

type Http struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type Grpc struct {
	Service string `json:"service"`
	Method  string `json:"method"`
	Path    string `json:"path"`
}

type Backend struct {
	CustomPath  string         `json:"custom_path"`
	Headers     varsModel.Vars `json:"headers"`
	QueryParams varsModel.Vars `json:"query_params"`
}

type ListReq struct {
	commonModel.ListParams

	AppId  *string
	Active *bool
}
