package gw

import (
	"fmt"
	"time"

	handlerHttp "github.com/rendau/ruto/internal/service/gw/handler/http"
	"github.com/rendau/ruto/internal/service/gw/jwk"
	"github.com/rendau/ruto/internal/service/gw/model/config"
	localHttp "github.com/rendau/ruto/internal/service/gw/server/http"
)

type Service struct {
	server *localHttp.Service
	jwk    *jwk.Service
}

func New(serverPort int) *Service {
	return &Service{
		server: localHttp.New(serverPort),
		jwk:    jwk.New(),
	}
}

func (s *Service) Run() {
	s.server.Run()
}

func (s *Service) Stop(timeout time.Duration) error {
	return s.server.Stop(timeout)
}

func (s *Service) SetConfig(conf *config.Root) error {
	httpHandler, err := handlerHttp.New(conf)
	if err != nil {
		return fmt.Errorf("handlerHttp.New: %w", err)
	}

	s.server.SetHandler(httpHandler)

	return nil
}
