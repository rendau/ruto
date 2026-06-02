package app

type SwaggerEndpoint struct {
	Method string
	Path   string
}

type SwaggerEndpointsDiff struct {
	Unregistered      []SwaggerEndpoint
	RegisteredInvalid []SwaggerEndpoint
}

type GrpcReflectionEndpoint struct {
	Service string
	Method  string
	Path    string
}
