package migrate

type authByRefreshTokenReq struct {
	RefreshToken string `json:"refresh_token"`
}

type authByRefreshTokenRep struct {
	AccessToken string `json:"access_token"`
}

type paginatedListRep[T any] struct {
	Results []T `json:"results"`
}

type legacyRealm struct {
	Id   string          `json:"id"`
	Data legacyRealmData `json:"data"`
}

type legacyRealmData struct {
	Name          string             `json:"name"`
	PublicBaseURL string             `json:"public_base_url"`
	CorsConf      legacyRealmCors    `json:"cors_conf"`
	JWTConf       legacyRealmJWTConf `json:"jwt_conf"`
}

type legacyRealmCors struct {
	Enabled          bool     `json:"enabled"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           string   `json:"max_age"`
	AllowOrigins     []string `json:"allow_origins"`
	AllowMethods     []string `json:"allow_methods"`
	AllowHeaders     []string `json:"allow_headers"`
}

type legacyRealmJWTConf struct {
	JWKURL string `json:"jwk_url"`
}

type legacyApp struct {
	Id      string        `json:"id"`
	RealmId string        `json:"realm_id"`
	Active  bool          `json:"active"`
	Data    legacyAppData `json:"data"`
}

type legacyAppData struct {
	Name        string               `json:"name"`
	Path        string               `json:"path"`
	BackendBase legacyAppBackendBase `json:"backend_base"`
}

type legacyAppBackendBase struct {
	Host string `json:"host"`
	Path string `json:"path"`
}

type legacyEndpoint struct {
	Id     string             `json:"id"`
	AppId  string             `json:"app_id"`
	Active bool               `json:"active"`
	Data   legacyEndpointData `json:"data"`
}

type legacyEndpointData struct {
	Method        string                `json:"method"`
	Path          string                `json:"path"`
	Backend       legacyEndpointBackend `json:"backend"`
	JWTValidation legacyEndpointJWTAuth `json:"jwt_validation"`
	IPValidation  legacyEndpointIPAuth  `json:"ip_validation"`
}

type legacyEndpointBackend struct {
	CustomPath bool   `json:"custom_path"`
	Path       string `json:"path"`
}

type legacyEndpointJWTAuth struct {
	Enabled bool     `json:"enabled"`
	Roles   []string `json:"roles"`
}

type legacyEndpointIPAuth struct {
	Enabled    bool     `json:"enabled"`
	AllowedIPs []string `json:"allowed_ips"`
}

type legacyJwkRep struct {
	Keys []legacyJwkKey `json:"keys"`
}

type legacyJwkKey struct {
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}
