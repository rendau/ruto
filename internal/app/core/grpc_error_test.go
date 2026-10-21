package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rendau/ruto/internal/errs"
)

func TestErrorStatusCodes(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantHTTP int
		wantCode string
	}{
		{"not authorized", fmt.Errorf("uc: %w", errs.NotAuthorized), http.StatusUnauthorized, errs.NotAuthorized.Error()},
		{"no permission", errs.NoPermission, http.StatusForbidden, errs.NoPermission.Error()},
		{"no permission with desc", errs.ErrFull{Err: errs.NoPermission, Desc: "app is not yours"}, http.StatusForbidden, errs.NoPermission.Error()},
		{"business error", errs.ObjectNotFound, http.StatusBadRequest, errs.ObjectNotFound.Error()},
		{"unexpected error", fmt.Errorf("boom"), http.StatusBadRequest, errs.ServiceNA.Error()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, grpcErr := GrpcInterceptorError()(
				context.Background(),
				nil,
				&grpc.UnaryServerInfo{FullMethod: "/test"},
				func(context.Context, any) (any, error) { return nil, tt.err },
			)
			require.Error(t, grpcErr)

			handler, err := GrpcGatewayCreateHandler(func(mux *runtime.ServeMux) error {
				return mux.HandlePath(http.MethodGet, "/fail", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
					runtime.HTTPError(r.Context(), mux, &runtime.JSONPb{}, w, r, grpcErr)
				})
			})
			require.NoError(t, err)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/fail", nil))
			require.Equal(t, tt.wantHTTP, rec.Code)

			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, tt.wantCode, body["code"])
		})
	}
}
