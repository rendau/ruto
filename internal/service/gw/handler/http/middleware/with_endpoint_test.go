package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rendau/ruto/internal/model/config"
)

func TestNewWithEndpoint(t *testing.T) {
	expected := &config.Endpoint{
		Id:     "users-list",
		Method: http.MethodGet,
		Path:   "users",
	}

	var actual *config.Endpoint
	handler := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			endpoint := EndpointFromRequest(r)
			if endpoint == nil {
				t.Fatalf("EndpointFromRequest() found = false, want true")
			}
			actual = endpoint
			w.WriteHeader(http.StatusNoContent)
		}),
		NewWithEndpoint(expected),
	)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	if rw.Code != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d want %d", rw.Code, http.StatusNoContent)
	}
	if actual != expected {
		t.Fatalf("unexpected endpoint: got %p want %p", actual, expected)
	}
}

func TestEndpointFromRequest_Empty(t *testing.T) {
	endpoint := EndpointFromRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if endpoint != nil {
		t.Fatalf("EndpointFromRequest() found = true, want false")
	}
}
