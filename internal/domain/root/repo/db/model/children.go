package model

import domModel "github.com/rendau/ruto/internal/domain/root/model"

type Cors struct {
	Enabled          bool     `json:"enabled"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           string   `json:"max_age"`
	AllowOrigins     []string `json:"allow_origins"`
	AllowMethods     []string `json:"allow_methods"`
	AllowHeaders     []string `json:"allow_headers"`
}

type Jwt struct {
	JwkUrl string `json:"jwk_url"`
}

func EncodeCors(v *Cors) *domModel.Cors {
	if v == nil {
		return nil
	}

	return &domModel.Cors{
		Enabled:          v.Enabled,
		AllowCredentials: v.AllowCredentials,
		MaxAge:           v.MaxAge,
		AllowOrigins:     v.AllowOrigins,
		AllowMethods:     v.AllowMethods,
		AllowHeaders:     v.AllowHeaders,
	}
}

func DecodeCors(v *domModel.Cors) *Cors {
	if v == nil {
		return nil
	}

	return &Cors{
		Enabled:          v.Enabled,
		AllowCredentials: v.AllowCredentials,
		MaxAge:           v.MaxAge,
		AllowOrigins:     v.AllowOrigins,
		AllowMethods:     v.AllowMethods,
		AllowHeaders:     v.AllowHeaders,
	}
}

func EncodeJwt(v *Jwt, _ int) *domModel.Jwt {
	if v == nil {
		return nil
	}

	return &domModel.Jwt{
		JwkUrl: v.JwkUrl,
	}
}

func DecodeJwt(v *domModel.Jwt, _ int) *Jwt {
	if v == nil {
		return nil
	}

	return &Jwt{
		JwkUrl: v.JwkUrl,
	}
}
