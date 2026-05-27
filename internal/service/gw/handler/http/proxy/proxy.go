package proxy

import (
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/request"
)

func NewProxy(app *appModel.App) http.Handler {
	backendUrl := app.Backend.ParsedUrl

	proxy := &httputil.ReverseProxy{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext: (&net.Dialer{
				Timeout: 2 * time.Second,
			}).DialContext,
			DisableCompression:  true,
			TLSHandshakeTimeout: 2 * time.Second,
			MaxIdleConnsPerHost: 100,
		},
		Rewrite: func(r *httputil.ProxyRequest) {
			ctxReq := request.Extract(r.In.Context())
			if ctxReq == nil {
				slog.Error("request.Extract() error", "error", "Request/Endpoint not found in context")
				r.SetURL(&url.URL{Scheme: "http", Host: "invalid-host"})
				return
			}
			endpoint := ctxReq.Endpoint

			slog.Debug("gw access",
				"method", r.In.Method,
				"path", r.In.URL.Path,
				"app", ctxReq.App.Name,
			)

			r.SetURL(backendUrl)

			if endpoint.Backend.CustomPath != "" {
				r.Out.URL.Path = backendUrl.JoinPath(endpoint.Backend.CustomPath).Path
			}

			// r.SetXForwarded()
		},
		// ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
		// },
	}

	return proxy
}
