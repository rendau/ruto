package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rateLimitModel "github.com/rendau/ruto/internal/domain/ratelimit/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/rw_wrapper"
	"github.com/rendau/ruto/internal/service/gw/service/clientip"
	"github.com/rendau/ruto/internal/service/gw/service/ratelimit"
)

func TestRateLimit_RejectionIsGatewaysOwnResponse(t *testing.T) {
	limits := ratelimit.NewService(ratelimit.New(100), clientip.New(nil, nil))
	limiter := limits.Endpoint(nil, &domAppModel.App{Id: "a"}, &domEndpointModel.Endpoint{
		Id: "e",
		RateLimit: rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{
			{Key: "ip", Requests: 1, Period: "1m", ParsedPeriod: time.Minute, Scope: "endpoint:e"},
		}},
	}, "GET /a")
	require.NotNil(t, limiter)

	h := NewRateLimit(limiter)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	serve := func() (*httptest.ResponseRecorder, *rw_wrapper.Wrapper) {
		rec := httptest.NewRecorder()
		rw := rw_wrapper.New(rec)
		h.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/a", nil))
		return rec, rw
	}

	rec, rw := serve()
	require.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, rw.IsExpected())

	rec, rw = serve()
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.True(t, rw.IsExpected(), "logged as an own response, like auth 401")
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}
