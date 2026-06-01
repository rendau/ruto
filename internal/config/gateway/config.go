package gateway

import (
	"errors"
	"os"

	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
)

var Conf = struct {
	Debug    bool   `env:"DEBUG" envDefault:"false"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	SystemPort int `env:"SYSTEM_PORT" envDefault:"3003"`

	HttpPort int `env:"HTTP_PORT"`
	GrpcPort int `env:"GRPC_PORT"`

	LogRequests bool `env:"LOG_REQUESTS" envDefault:"false"`

	CoreGrpcAddress string `env:"CORE_GRPC_ADDRESS"`

	TrustedProxyAddresses []string `env:"TRUSTED_PROXY_ADDRESSES" envSeparator:","`
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
