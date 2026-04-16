package http

import (
	"github.com/rendau/ruto/internal/model/config"
)

type Service struct {
}

func New() *Service {
	return &Service{}
}

func (s *Service) SetConfig(conf *config.Root) error {
	return nil
}
