package http

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"

	jwtv5 "github.com/golang-jwt/jwt/v5"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/jwt"
)

type benchmarkJWKGetter struct {
	publicKey *rsa.PublicKey
	alg       string
}

func (g *benchmarkJWKGetter) GetPublicKey(string) (*rsa.PublicKey, string) {
	return g.publicKey, g.alg
}

type benchmarkResponseWriter struct {
	headers http.Header
	code    int
}

func newBenchmarkResponseWriter() *benchmarkResponseWriter {
	return &benchmarkResponseWriter{
		headers: make(http.Header, 16),
	}
}

func (w *benchmarkResponseWriter) Header() http.Header {
	return w.headers
}

func (w *benchmarkResponseWriter) Write(bytes []byte) (int, error) {
	return len(bytes), nil
}

func (w *benchmarkResponseWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

func (w *benchmarkResponseWriter) Reset() {
	clear(w.headers)
	w.code = 0
}

func BenchmarkServiceServeHTTP(b *testing.B) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		b.Fatalf("rsa.GenerateKey() error: %v", err)
	}
	publicKey := privateKey.PublicKey
	kid := "kid-1"
	alg := jwtv5.SigningMethodRS256.Alg()

	jwtTokenObj := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, jwtv5.MapClaims{
		"sub":   "u1",
		"roles": []string{"admin"},
	})
	jwtTokenObj.Header["kid"] = kid

	jwtToken, err := jwtTokenObj.SignedString(privateKey)
	if err != nil {
		b.Fatalf("SignedString() error: %v", err)
	}

	jwkGetter := &benchmarkJWKGetter{
		publicKey: &publicKey,
		alg:       alg,
	}

	benchCases := []struct {
		name     string
		endpoint *endpointModel.Endpoint
		req      func() *http.Request
		jwk      jwt.JwkGetterI
	}{
		{
			name: "no_auth",
			endpoint: &endpointModel.Endpoint{
				Active: true,
				Method: http.MethodGet,
				Path:   "users",
			},
			req: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "https://public.example/api/users", nil)
			},
		},
		{
			name: "api_key",
			endpoint: &endpointModel.Endpoint{
				Active: true,
				Method: http.MethodGet,
				Path:   "users",
				Auth: authModel.Auth{
					Enabled: true,
					Methods: []*authModel.AuthMethod{
						{
							APIKey: &authModel.AuthMethodAPIKey{
								Header: "X-API-Key",
								Keys:   []string{"k-1"},
							},
						},
					},
				},
			},
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://public.example/api/users", nil)
				req.Header.Set("X-API-Key", "k-1")
				return req
			},
		},
		{
			name: "jwt_rs256",
			endpoint: &endpointModel.Endpoint{
				Active: true,
				Method: http.MethodGet,
				Path:   "users",
				Auth: authModel.Auth{
					Enabled: true,
					Methods: []*authModel.AuthMethod{
						{
							JWT: &authModel.AuthMethodJWT{
								Kid:   kid,
								Roles: []string{"admin"},
							},
						},
					},
				},
			},
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://public.example/api/users", nil)
				req.Header.Set("Authorization", "Bearer "+jwtToken)
				return req
			},
			jwk: jwkGetter,
		},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			s, err := New(&rootModel.Root{
				BaseUrl: "https://public.example",
				Apps: []*appModel.App{
					{
						Active:     true,
						PathPrefix: "api",
						Backend: appModel.AppBackend{
							Url: backend.URL + "/svc",
						},
						Endpoints: []*endpointModel.Endpoint{
							bc.endpoint,
						},
					},
				},
			}, bc.jwk)
			if err != nil {
				b.Fatalf("New() error: %v", err)
			}

			req := bc.req()
			rw := newBenchmarkResponseWriter()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				rw.Reset()
				s.ServeHTTP(rw, req)

				if rw.code != http.StatusNoContent {
					b.Fatalf("unexpected status code: %d", rw.code)
				}
			}
		})
	}
}
