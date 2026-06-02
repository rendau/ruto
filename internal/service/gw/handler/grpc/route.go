package grpc

import (
	"github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	serviceAuth "github.com/rendau/ruto/internal/service/gw/service/auth"
	serviceLog "github.com/rendau/ruto/internal/service/gw/service/log"
	serviceMetrics "github.com/rendau/ruto/internal/service/gw/service/metrics"
)

type route struct {
	app      *model.App
	endpoint *domEndpointModel.Endpoint
	auth     *serviceAuth.Service
	log      *serviceLog.Service
	metrics  *serviceMetrics.Service
}

type routeCtxKey struct{}
