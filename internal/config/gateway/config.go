package gateway

import (
	"errors"
	"os"
	"time"

	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
)

var Conf = struct {
	Debug    bool   `env:"DEBUG" envDefault:"false"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	SystemPort int `env:"SYSTEM_PORT" envDefault:"3003"`

	HttpPort int `env:"HTTP_PORT"`
	GrpcPort int `env:"GRPC_PORT"`

	CoreGrpcAddress string `env:"CORE_GRPC_ADDRESS"`

	// below the keep-alive timeout of every backend (Go net/http: 60s and more,
	// uvicorn and Node.js: 5s), see proxy.NewTransport
	BackendIdleConnTimeout time.Duration `env:"BACKEND_IDLE_CONN_TIMEOUT" envDefault:"4s"`

	TrustedProxyAddresses []string `env:"TRUSTED_PROXY_ADDRESSES" envSeparator:","`

	// Cloudflare's networks: a request from them gets its client address from
	// Cf-Connecting-Ip; a zero refresh keeps the built-in list, see
	// infra/cloudflare
	CloudflareIPsURL     string        `env:"CLOUDFLARE_IPS_URL" envDefault:"https://api.cloudflare.com/client/v4/ips"`
	CloudflareIPsRefresh time.Duration `env:"CLOUDFLARE_IPS_REFRESH" envDefault:"24h"`

	// memory ceiling for responses of endpoints with server cache, see gw/service/cache
	CacheMaxSizeMb     int           `env:"CACHE_MAX_SIZE_MB" envDefault:"256"`
	CacheSweepInterval time.Duration `env:"CACHE_SWEEP_INTERVAL" envDefault:"30s"`

	// ceiling on rate-limit counters (client × rule), ~85 bytes each, so the
	// default takes up to ~45 MB; at the ceiling the least used are evicted,
	// see gw/service/ratelimit
	RateLimitMaxKeys int `env:"RATE_LIMIT_MAX_KEYS" envDefault:"500000"`
}{}

func init() {
	err := godotenv.Load(".env.gateway")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
	}

	if err = env.Parse(&Conf); err != nil {
		panic(err)
	}
}
