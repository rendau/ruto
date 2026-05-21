package config

import (
	"github.com/caarlos0/env/v9"
	_ "github.com/joho/godotenv/autoload"
)

var Conf = struct {
	Namespace string `env:"NAMESPACE" envDefault:"example.com"`
	Debug     bool   `env:"DEBUG" envDefault:"false"`
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`

	WithMetrics   bool   `env:"WITH_METRICS" envDefault:"false"`
	WithTracing   bool   `env:"WITH_TRACING" envDefault:"false"`
	JaegerAddress string `env:"JAEGER_ADDRESS"`

	GrpcPort int  `env:"GRPC_PORT" envDefault:"5050"`
	HttpPort int  `env:"HTTP_PORT" envDefault:"80"`
	HttpCors bool `env:"HTTP_CORS" envDefault:"false"`

	AdminJWTSecret string `env:"ADMIN_JWT_SECRET"`

	PgDsn string `env:"PG_DSN"`

	GwPort              int    `env:"GW_PORT"`
	SnapshotGrpcAddress string `env:"SNAPSHOT_GRPC_ADDRESS" envDefault:"localhost:5050"`

	LegacyDMBaseURL      string `env:"LEGACY_DM_BASE_URL"`
	LegacyDMRefreshToken string `env:"LEGACY_DM_REFRESH_TOKEN"`
}{}

func init() {
	if err := env.Parse(&Conf); err != nil {
		panic(err)
	}
}
