package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/infra/clientip"
	"github.com/rendau/ruto/internal/service/gw/service/clientstat"
)

// A preflight the gateway answers itself never reaches an endpoint and its
// rate limits, so the client stats mark it: the rates compared to the limits
// leave it out.
func TestCors_PreflightCountedInClientStats(t *testing.T) {
	serve := func(cors rootModel.RootCors, method, origin string) (int, bool, *clientstat.Item) {
		snapshot := rootModel.NewEmpty()
		snapshot.Cors = cors
		require.NoError(t, snapshot.Normalize())

		collector := clientstat.NewCollector(100)
		meter := clientstat.New(collector, clientip.New(nil, nil)).Meter(snapshot)
		reached := false
		h := Chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reached = true
			w.WriteHeader(http.StatusOK)
		}), NewClientStat(meter), NewCors(snapshot.Cors))

		req := httptest.NewRequest(method, "/shop/products/1", nil)
		req.RemoteAddr = "203.0.113.7:1000"
		if origin != "" {
			req.Header.Set("Origin", origin)
			req.Header.Set("Access-Control-Request-Method", http.MethodGet)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		items := collector.Drain()
		require.Len(t, items, 1)
		return rec.Code, reached, items[0]
	}

	on := rootModel.RootCors{Enabled: true, AllowOrigins: []string{"https://shop.mdev.kz"}}

	code, reached, item := serve(on, http.MethodOptions, "https://shop.mdev.kz")
	assert.Equal(t, http.StatusNoContent, code)
	assert.False(t, reached)
	assert.Equal(t, &clientstat.Item{Ip: "203.0.113.7", Requests: 1, Preflights: 1}, item)

	code, reached, item = serve(on, http.MethodOptions, "https://evil.example")
	assert.Equal(t, http.StatusForbidden, code)
	assert.False(t, reached)
	assert.Equal(t, int64(1), item.Preflights, "a refused preflight is answered by the gateway too")

	code, reached, item = serve(on, http.MethodGet, "https://shop.mdev.kz")
	assert.Equal(t, http.StatusOK, code)
	assert.True(t, reached)
	assert.Zero(t, item.Preflights)

	code, reached, item = serve(rootModel.RootCors{Enabled: false}, http.MethodOptions, "https://shop.mdev.kz")
	assert.Equal(t, http.StatusOK, code)
	assert.True(t, reached, "without Root CORS the preflight goes on like any request")
	assert.Zero(t, item.Preflights)
}
