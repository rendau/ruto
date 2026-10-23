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

	// memory ceiling for responses of endpoints with server cache, see gw/service/cache
	CacheMaxSizeMb     int           `env:"CACHE_MAX_SIZE_MB" envDefault:"256"`
	CacheSweepInterval time.Duration `env:"CACHE_SWEEP_INTERVAL" envDefault:"30s"`
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
