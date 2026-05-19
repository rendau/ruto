package stats

type Stats struct {
	AppsTotal         int64
	AppsActive        int64
	AppsInactive      int64
	EndpointsTotal    int64
	EndpointsActive   int64
	EndpointsInactive int64
	UsersTotal        int64
	UsersActive       int64
	UsersAdmin        int64
	RootJWTProviders  int64
	RootAuthEnabled   bool
	RootCorsEnabled   bool
	Methods           []MethodStats
}

type MethodStats struct {
	Method string
	Total  int64
	Active int64
}
