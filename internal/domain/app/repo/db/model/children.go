package model

import domModel "github.com/rendau/ruto/internal/domain/app/model"

type Backend struct {
	Url string `json:"url"`
}

func EncodeBackend(v *Backend) *domModel.Backend {
	if v == nil {
		return nil
	}
	return &domModel.Backend{
		Url: v.Url,
	}
}

func DecodeBackend(v *domModel.Backend) *Backend {
	if v == nil {
		return nil
	}
	return &Backend{
		Url: v.Url,
	}
}
