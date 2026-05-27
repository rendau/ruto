package core

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

	GrpcPort int  `env:"GRPC_PORT" envDefault:"5050"`
	HttpPort int  `env:"HTTP_PORT" envDefault:"80"`
	HttpCors bool `env:"HTTP_CORS" envDefault:"false"`

	AdminJWTSecret string `env:"ADMIN_JWT_SECRET"`

	PgDsn string `env:"PG_DSN"`

	RedisAddr     string `env:"REDIS_ADDR"`
	RedisDB       int    `env:"REDIS_DB" envDefault:"0"`
	RedisPassword string `env:"REDIS_PASSWORD"`

	LegacyDMBaseURL      string `env:"LEGACY_DM_BASE_URL"`
	LegacyDMRefreshToken string `env:"LEGACY_DM_REFRESH_TOKEN"`
}{}

func init() {
	err := godotenv.Load(".env.core")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
	}

	if err = env.Parse(&Conf); err != nil {
		panic(err)
	}
}
