package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	loggingModel "github.com/rendau/ruto/internal/domain/logging/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/rw_wrapper"
)

func captureLogs(t *testing.T, f func()) string {
	t.Helper()

	var buf strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	f()
	return buf.String()
}

func runRequestLog(ep *domEndpointModel.Endpoint, backend http.HandlerFunc, req *http.Request) {
	runRequestLogWithOwnResponseErrors(ep, backend, req, false)
}

func runRequestLogWithOwnResponseErrors(ep *domEndpointModel.Endpoint, backend http.HandlerFunc, req *http.Request, logOwnResponseErrors bool) {
	app := &domAppModel.App{Name: "test-app"}
	handler := NewRequestLog(app, ep, "/path", logOwnResponseErrors)(backend)
	handler.ServeHTTP(httptest.NewRecorder(), req)
}

func TestRequestLogMasksSensitiveAndTruncates(t *testing.T) {
	ep := &domEndpointModel.Endpoint{
		Logging: loggingModel.Logging{
			Level:        "all",
			Headers:      true,
			QueryParams:  true,
			ReqBody:      true,
			RespBody:     true,
			ReqBodyLimit: 5,
		},
		Auth: authModel.Auth{
			Methods: []*authModel.AuthMethod{
				{APIKey: &authModel.AuthMethodAPIKey{Header: "X-Custom-Key"}},
			},
		},
	}

	backend := func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte("response-body-content"))
	}

	req := httptest.NewRequest(http.MethodPost, "/path?api-key=secretquery&normal=ok", strings.NewReader("hello world"))
	req.Header.Set("Authorization", "Bearer topsecret")
	req.Header.Set("Api-Key", "secretheader")
	req.Header.Set("X-Custom-Key", "configuredsecret")
	req.Header.Set("X-Normal", "visible")

	out := captureLogs(t, func() { runRequestLog(ep, backend, req) })

	for _, secret := range []string{"topsecret", "secretheader", "configuredsecret", "secretquery"} {
		if strings.Contains(out, secret) {
			t.Fatalf("log leaked sensitive value %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, "visible") {
		t.Fatalf("non-sensitive header should be logged: %s", out)
	}
	if !strings.Contains(out, "truncated") {
		t.Fatalf("oversized request body should be truncated: %s", out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("request body prefix should be logged: %s", out)
	}
}

func TestRequestLogLevelErrorSkipsSuccess(t *testing.T) {
	ep := &domEndpointModel.Endpoint{
		Logging: loggingModel.Logging{Level: "error"},
	}
	backend := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
	req := httptest.NewRequest(http.MethodGet, "/path", nil)

	out := captureLogs(t, func() { runRequestLog(ep, backend, req) })
	if strings.TrimSpace(out) != "" {
		t.Fatalf("level=error must not log successful request, got: %s", out)
	}
}

func TestRequestLogLevelNoneLogsNothing(t *testing.T) {
	ep := &domEndpointModel.Endpoint{
		Logging: loggingModel.Logging{Level: "none"},
	}
	backend := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	req := httptest.NewRequest(http.MethodGet, "/path", nil)

	out := captureLogs(t, func() { runRequestLog(ep, backend, req) })
	if strings.TrimSpace(out) != "" {
		t.Fatalf("level=none must not log anything, even errors, got: %s", out)
	}
}

func TestRequestLogLevelErrorLogsFailure(t *testing.T) {
	ep := &domEndpointModel.Endpoint{
		Logging: loggingModel.Logging{Level: "error"},
	}
	backend := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	req := httptest.NewRequest(http.MethodGet, "/path", nil)

	out := captureLogs(t, func() { runRequestLog(ep, backend, req) })
	if !strings.Contains(out, "500") {
		t.Fatalf("level=error must log failed request, got: %s", out)
	}
}

// expectedBackend mimics the auth middleware: it marks the response as the
// gateway's own expected non-2xx and writes a 401.
func expectedBackend(w http.ResponseWriter, _ *http.Request) {
	if rw, ok := w.(*rw_wrapper.Wrapper); ok {
		rw.MarkExpected()
	}
	w.WriteHeader(http.StatusUnauthorized)
}

func TestRequestLogOwnResponseErrorsDisabledSkipsOwn401(t *testing.T) {
	ep := &domEndpointModel.Endpoint{
		Logging: loggingModel.Logging{Level: "all"},
	}
	req := httptest.NewRequest(http.MethodGet, "/path", nil)

	out := captureLogs(t, func() { runRequestLogWithOwnResponseErrors(ep, expectedBackend, req, false) })
	if strings.TrimSpace(out) != "" {
		t.Fatalf("disabled log_own_response_errors must skip own 401 even at level=all, got: %s", out)
	}
}

func TestRequestLogOwnResponseErrorsEnabledLogsOwn401(t *testing.T) {
	ep := &domEndpointModel.Endpoint{
		Logging: loggingModel.Logging{Level: "error"},
	}
	req := httptest.NewRequest(http.MethodGet, "/path", nil)

	out := captureLogs(t, func() { runRequestLogWithOwnResponseErrors(ep, expectedBackend, req, true) })
	if !strings.Contains(out, "401") {
		t.Fatalf("enabled log_own_response_errors must log own 401 at level=error, got: %s", out)
	}
}
