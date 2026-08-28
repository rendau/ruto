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

	AppSwaggerDiscoveryOnStart bool `env:"APP_SWAGGER_DISCOVERY_ON_START" envDefault:"false"`

	// monitoring: external Prometheus / log-store the admin charts are read from
	PrometheusURL    string `env:"PROMETHEUS_URL"`
	MetricsNamespace string `env:"METRICS_NAMESPACE" envDefault:"company"`

	LokiURL      string `env:"LOKI_URL"`
	LokiSelector string `env:"LOKI_SELECTOR" envDefault:"{app=\"ruto-gateway\"}"`
	LokiOrgID    string `env:"LOKI_ORG_ID"`

	GraylogURL      string `env:"GRAYLOG_URL"`
	GraylogAPIToken string `env:"GRAYLOG_API_TOKEN"`
	GraylogStreamID string `env:"GRAYLOG_STREAM_ID"`
	GraylogQuery    string `env:"GRAYLOG_QUERY"`
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
