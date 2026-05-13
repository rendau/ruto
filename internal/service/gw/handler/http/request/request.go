package request

import (
	"context"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/jwk"
)

type Request struct {
	Root       *rootModel.Root
	App        *appModel.App
	Endpoint   *endpointModel.Endpoint
	JwkService JwkServiceI
}

type JwkServiceI interface {
	Get(kid string) *jwk.Item
}

type contextKey struct{}

func Inject(ctx context.Context, req *Request) context.Context {
	return context.WithValue(ctx, contextKey{}, req)
}

func Extract(ctx context.Context) *Request {
	req, ok := ctx.Value(contextKey{}).(*Request)
	if ok {
		return req
	}
	return nil
}
