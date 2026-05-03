package model

type Main struct {
	PublicBaseUrl string
	Cors          *Cors
	Jwt           []*Jwt
}

type Edit struct {
	PublicBaseUrl *string
	Cors          *Cors
	Jwt           *[]*Jwt
}

type Cors struct {
	Enabled          bool
	AllowCredentials bool
	MaxAge           string
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
}

type Jwt struct {
	JwkUrl string
}

func NewEmpty() *Main {
	return &Main{
		Cors: &Cors{
			AllowOrigins: []string{},
			AllowMethods: []string{},
			AllowHeaders: []string{},
		},
		Jwt: []*Jwt{},
	}
}
