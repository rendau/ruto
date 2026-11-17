package core

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

	// built-in MCP server for AI agents (api-key auth, own port)
	McpEnabled bool `env:"MCP_ENABLED" envDefault:"false"`
	McpPort    int  `env:"MCP_PORT" envDefault:"5060"`

	// OAuth for MCP clients that connect by URL (Claude connectors); api keys
	// keep working alongside. McpPublicURL is the MCP server URL as clients
	// see it, McpOAuthAdminURL is where the admin UI (consent page) is opened.
	McpOAuthEnabled      bool          `env:"MCP_OAUTH_ENABLED" envDefault:"false"`
	McpPublicURL         string        `env:"MCP_PUBLIC_URL"`
	McpOAuthAdminURL     string        `env:"MCP_OAUTH_ADMIN_URL"`
	McpOAuthRedirectURIs []string      `env:"MCP_OAUTH_REDIRECT_URIS" envSeparator:"," envDefault:"https://claude.ai/api/mcp/auth_callback,https://claude.com/api/mcp/auth_callback,http://localhost,http://127.0.0.1"`
	McpOAuthAccessTTL    time.Duration `env:"MCP_OAUTH_ACCESS_TTL" envDefault:"1h"`
	McpOAuthRefreshTTL   time.Duration `env:"MCP_OAUTH_REFRESH_TTL" envDefault:"720h"`

	// change requests proposed over MCP
	ChangeRequestTTL           time.Duration `env:"CHANGE_REQUEST_TTL" envDefault:"168h"`
	ChangeRequestRetention     time.Duration `env:"CHANGE_REQUEST_RETENTION" envDefault:"720h"`
	ChangeRequestMaxPendingKey int64         `env:"CHANGE_REQUEST_MAX_PENDING_PER_KEY" envDefault:"20"`
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
