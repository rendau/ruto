package system

import (
	"time"
)

type Gateways struct {
	PublishedVersion string
	Gateways         []*Gateway
}

// Gateway is a gateway as core sees it from its heartbeats and its notification
// stream.
type Gateway struct {
	HostName    string
	Status      string // usecaseGateway.Status*
	Connected   bool   // holds the notification stream to core
	Version     string
	UpToDate    bool
	Lagging     bool // on an old version longer than LagAfter since core saw the publish
	Silent      bool // connected, but its heartbeats stopped
	LastApplyAt *time.Time
	LastSeenAt  *time.Time
	LastError   string
}

// Routes is the published configuration reduced to what routes a request:
// auth, variables, backend headers and query params are never copied here.
type Routes struct {
	Version string
	BaseUrl string
	Apps    []*RouteApp
}

type RouteApp struct {
	Id                 string
	Name               string
	Active             bool
	PathPrefix         string
	ExcludeFromMetrics bool
	BackendUrl         string
	GrpcUrl            string
	Endpoints          []*RouteEndpoint
}

type RouteEndpoint struct {
	Id       string
	Active   bool
	Type     string
	Method   string
	Path     string
	GrpcPath string
}
