package proxy

import (
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/rendau/ruto/internal/model/config"
	localContext "github.com/rendau/ruto/internal/service/gw/handler/http/context"
)

func NewProxy(app *config.App) http.Handler {
	backendUrl := app.Backend.Url

	proxy := &httputil.ReverseProxy{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext: (&net.Dialer{
				Timeout: 1 * time.Second,
			}).DialContext,
			DisableCompression:  true,
			TLSHandshakeTimeout: 1 * time.Second,
			MaxIdleConnsPerHost: 50,
		},
		Rewrite: func(r *httputil.ProxyRequest) {
			endpoint := localContext.ExtractEndpoint(r.In.Context())
			if endpoint == nil {
				slog.Error("context.ExtractEndpoint() error", "error", "Endpoint not found in context")
				r.SetURL(&url.URL{Scheme: "http", Host: "invalid-host"})
				return
			}

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
