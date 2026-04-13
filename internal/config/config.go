package config

import (
	"github.com/caarlos0/env/v9"
	_ "github.com/joho/godotenv/autoload"
)

var Conf = struct {
	Namespace string `env:"NAMESPACE" envDefault:"example.com"`
	Debug     bool   `env:"DEBUG" envDefault:"false"`
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	HttpPort  string `env:"HTTP_PORT" envDefault:"80"`
	HttpCors  bool   `env:"HTTP_CORS" envDefault:"false"`
	PgDsn     string `env:"PG_DSN"`
}{}

func init() {
	if err := env.Parse(&Conf); err != nil {
		panic(err)
	}
}
