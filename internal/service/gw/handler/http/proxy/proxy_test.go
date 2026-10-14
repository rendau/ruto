package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
)

func TestRewriteLocation(t *testing.T) {
	tests := []struct {
		name       string
		backendUrl string
		prefix     string
		location   string
		want       string
	}{
		// backend without a path: the prefix is prepended, as before
		{name: "no path: host-relative", backendUrl: "http://back", prefix: "/account", location: "/login", want: "/account/login"},
		{name: "no path: root", backendUrl: "http://back", prefix: "/account", location: "/", want: "/account/"},
		{name: "slash path: same as no path", backendUrl: "http://back/", prefix: "/account", location: "/login?next=%2Fme", want: "/account/login?next=%2Fme"},
		{name: "no path: path-relative untouched", backendUrl: "http://back", prefix: "/account", location: "login", want: "login"},
		{name: "no path: query-only untouched", backendUrl: "http://back", prefix: "/account", location: "?page=2", want: "?page=2"},
		{name: "no path: absolute on backend host", backendUrl: "http://back", prefix: "/account", location: "http://back/login", want: "/account/login"},
		{name: "no path: absolute on backend host without path", backendUrl: "http://back:8080", prefix: "/account", location: "http://back:8080", want: "/account/"},

		// B = /p, P = /p (mechta.kz)
		{name: "B=/p: same prefix", backendUrl: "http://back-sps/p", prefix: "/p", location: "/p/xyz", want: "/p/xyz"},
		{name: "B=/p: exact backend path", backendUrl: "http://back-sps/p", prefix: "/p", location: "/p", want: "/p"},
		{name: "B=/p: with query", backendUrl: "http://back-sps/p", prefix: "/p", location: "/p?x=1", want: "/p?x=1"},
		{name: "B=/p: root untouched", backendUrl: "http://back-sps/p", prefix: "/p", location: "/", want: "/"},
		{name: "B=/p: other path untouched", backendUrl: "http://back-sps/p", prefix: "/p", location: "/other", want: "/other"},
		{name: "B=/p: not a segment boundary", backendUrl: "http://back-sps/p", prefix: "/p", location: "/pxyz", want: "/pxyz"},
		{name: "B=/p/: trailing slash ignored", backendUrl: "http://back-sps/p/", prefix: "/p", location: "/p/xyz", want: "/p/xyz"},
		{name: "B=/p: absolute on backend host", backendUrl: "http://back-sps/p", prefix: "/p", location: "http://back-sps/p/xyz", want: "/p/xyz"},
		{name: "B=/p: https absolute on backend host", backendUrl: "http://back-sps/p", prefix: "/p", location: "https://BACK-SPS/p/xyz#top", want: "/p/xyz#top"},
		{name: "B=/p: protocol-relative on backend host", backendUrl: "http://back-sps/p", prefix: "/p", location: "//back-sps/p/xyz", want: "/p/xyz"},
		{name: "B=/p: absolute on backend host outside B", backendUrl: "http://back-sps/p", prefix: "/p", location: "http://back-sps/other", want: "/other"},

		// B = /api, P = /api/v1 (mechta.kz)
		{name: "B=/api: mapped to prefix", backendUrl: "http://back-sps/api", prefix: "/api/v1", location: "/api/orders/1", want: "/api/v1/orders/1"},
		{name: "B=/api: exact backend path", backendUrl: "http://back-sps/api", prefix: "/api/v1", location: "/api", want: "/api/v1"},
		{name: "B=/api: root untouched", backendUrl: "http://back-sps/api", prefix: "/api/v1", location: "/", want: "/"},
		{name: "B=/api: other path untouched", backendUrl: "http://back-sps/api", prefix: "/api/v1", location: "/other", want: "/other"},
		{name: "B=/api: absolute on backend host", backendUrl: "http://back-sps/api", prefix: "/api/v1", location: "http://back-sps/api/orders?id=1", want: "/api/v1/orders?id=1"},

		// other hosts and schemes are never touched
		{name: "absolute on other host", backendUrl: "http://back-sps/p", prefix: "/p", location: "https://mechta.kz/p/xyz", want: "https://mechta.kz/p/xyz"},
		{name: "protocol-relative on other host", backendUrl: "http://back", prefix: "/account", location: "//auth.example/login", want: "//auth.example/login"},
		{name: "other host with backend port mismatch", backendUrl: "http://back:8080", prefix: "/account", location: "http://back:9090/login", want: "http://back:9090/login"},
		{name: "non-http scheme", backendUrl: "http://back", prefix: "/account", location: "ftp://back/file", want: "ftp://back/file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backendUrl, err := url.Parse(tt.backendUrl)
			require.NoError(t, err)

			assert.Equal(t, tt.want, rewriteLocation(tt.location, backendUrl, tt.prefix))
		})
	}
}

func TestNewProxy_XForwarded(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		inHeaders  http.Header
		wantFor    string
		wantProto  string
		wantHost   string
	}{
		{
			name:       "no inbound headers: client ip, plain http, request host",
			remoteAddr: "203.0.113.7:51000",
			wantFor:    "203.0.113.7",
			wantProto:  "http",
			wantHost:   "gw.local",
		},
		{
			name:       "inbound headers from the edge proxy are kept, client ip appended",
			remoteAddr: "10.0.0.5:51000",
			inHeaders: http.Header{
				"X-Forwarded-For":   {"198.51.100.1"},
				"X-Forwarded-Proto": {"https"},
				"X-Forwarded-Host":  {"mechta.kz"},
			},
			wantFor:   "198.51.100.1, 10.0.0.5",
			wantProto: "https",
			wantHost:  "mechta.kz",
		},
		{
			name:       "several inbound X-Forwarded-For lines are joined",
			remoteAddr: "10.0.0.5:51000",
			inHeaders: http.Header{
				"X-Forwarded-For": {"198.51.100.1", "192.0.2.10"},
			},
			wantFor:   "198.51.100.1, 192.0.2.10, 10.0.0.5",
			wantProto: "http",
			wantHost:  "gw.local",
		},
	}

	for _, tt := range tests {
		for _, customPath := range []string{"", "internal/me"} {
			t.Run(tt.name+"/custom_path="+customPath, func(t *testing.T) {
				var got http.Header
				backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					got = r.Header.Clone()
				}))
				defer backend.Close()

				app := &appModel.App{PathPrefix: "/account", Backend: appModel.Backend{Url: backend.URL}}
				require.NoError(t, app.Normalize())

				req := httptest.NewRequest(http.MethodGet, "http://gw.local/profile", nil)
				req.RemoteAddr = tt.remoteAddr
				for name, values := range tt.inHeaders {
					req.Header[name] = values
				}

				NewProxy(app, customPath, NewTransport()).ServeHTTP(httptest.NewRecorder(), req)

				require.NotNil(t, got)
				assert.Equal(t, tt.wantFor, got.Get("X-Forwarded-For"))
				assert.Equal(t, tt.wantProto, got.Get("X-Forwarded-Proto"))
				assert.Equal(t, tt.wantHost, got.Get("X-Forwarded-Host"))
			})
		}
	}
}
