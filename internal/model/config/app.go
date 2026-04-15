package config

type App struct {
	Id         string
	PublicPath string
	Backend    AppBackend
}

type AppBackend struct {
	Host string
	Path string
}
