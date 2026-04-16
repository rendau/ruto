package config

import (
	"time"
)

type Root struct {
	PublicBaseUrl string
	Timeout       RootTimeout
	Cors          RootCors
	Jwt           []RootJwt
	Apps          []App
}

type RootTimeout struct {
	Global     time.Duration
	ReadHeader time.Duration
	Read       time.Duration
}

type RootCors struct {
	Enabled          bool
	AllowCredentials bool
	MaxAge           string
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
}

type RootJwt struct {
	JwkUrl        string
	Alg           string
	CacheDuration time.Duration
	RolesPath     string
}
