package config

type App struct {
	Id         string
	PublicPath string
	Backend    AppBackend
	Endpoints  []Endpoint
}

type AppBackend struct {
	Host string
	Path string
}
