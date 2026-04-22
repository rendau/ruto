package middleware

import (
	"net/http"

	localContext "github.com/rendau/ruto/internal/service/gw/handler/http/context"
	"github.com/rendau/ruto/internal/service/gw/model/config"
)

func NewWithEndpoint(ep *config.Endpoint) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(localContext.WithEndpoint(r.Context(), ep)))
		})
	}
}
