package middleware

import (
	"net/http"

	"github.com/rendau/ruto/internal/domain/config/model"
	localContext "github.com/rendau/ruto/internal/service/gw/handler/http/context"
)

func NewWithEndpoint(ep *model.Endpoint) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(localContext.WithEndpoint(r.Context(), ep)))
		})
	}
}
