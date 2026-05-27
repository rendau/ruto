package core

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/goccy/go-json"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/cors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	configCore "github.com/rendau/ruto/internal/config/core"
	"github.com/rendau/ruto/internal/errs"
)

func GrpcGatewayCreateHandler(muxHook func(*runtime.ServeMux) error) (http.Handler, error) {
	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   true,
				EmitUnpopulated: true,
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}),
		runtime.WithErrorHandler(func(_ context.Context, _ *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, _ *http.Request, err error) {
			var repBody []byte

			if st, ok := status.FromError(err); ok {
				if st.Code() == codes.NotFound {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`service path not found`))
					return
				}
				if st.Code() == codes.InvalidArgument && len(st.Details()) > 0 {
					var marshalErr error
					repBody, marshalErr = marshaler.Marshal(st.Details()[0])
					if marshalErr != nil {
						slog.Error("GRPC_GW: ErrorHandler: Failed to marshal", "error", marshalErr)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				}
			}

			if len(repBody) == 0 {
				obj := map[string]string{
					"code":    errs.ServiceNA.Error(),
					"message": err.Error(),
				}
				repBody, err = json.Marshal(obj)
				if err != nil {
					slog.Error("GRPC_GW: ErrorHandler: Failed to marshal", "error", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, err = io.Copy(w, bytes.NewReader(repBody))
			if err != nil {
				slog.Error("GRPC_GW: ErrorHandler: Failed to write response", "error", err)
			}
		}),
	)

	if muxHook != nil {
		err := muxHook(mux)
		if err != nil {
			return nil, fmt.Errorf("grpc-gateway: muxHook: %w", err)
		}
	}

	err := mux.HandlePath(http.MethodGet, "/healthcheck", func(w http.ResponseWriter, _ *http.Request, _ map[string]string) {
		w.WriteHeader(http.StatusOK)
	})
	if err != nil {
		return nil, fmt.Errorf("grpc-gateway: register healthcheck handler: %w", err)
	}

	docFS := http.StripPrefix("/docs/", http.FileServer(http.Dir("./docs")))
	err = mux.HandlePath(http.MethodGet, "/docs/{any}", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		docFS.ServeHTTP(w, r)
	})
	if err != nil {
		return nil, fmt.Errorf("grpc-gateway: register docs handler: %w", err)
	}
	err = mux.HandlePath(http.MethodGet, "/docs/{any}/{any}", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		docFS.ServeHTTP(w, r)
	})
	if err != nil {
		return nil, fmt.Errorf("grpc-gateway: register docs handler: %w", err)
	}

	handler := http.Handler(mux)

	if configCore.Conf.HttpCors {
		handler = cors.New(cors.Options{
			AllowOriginFunc: func(string) bool { return true },
			AllowedMethods: []string{
				http.MethodGet,
				http.MethodPut,
				http.MethodPost,
				http.MethodDelete,
			},
			AllowedHeaders: []string{
				"Accept",
				"Content-Type",
				"X-Requested-With",
				"Authorization",
			},
			AllowCredentials: true,
			MaxAge:           604800,
		}).Handler(handler)
	}

	handler = func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					slog.Error(
						"Recovered from panic",
						slog.Any("error", err),
						slog.Any("recovery_stacktrace", string(debug.Stack())),
					)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}()
			h.ServeHTTP(w, r)
		})
	}(handler)

	return handler, nil
}
