package proxy

import (
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
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

			r.SetURL(backendUrl)

			if endpoint.Backend.CustomPath != "" {
				r.Out.URL.Path = backendUrl.JoinPath(endpoint.Backend.CustomPath).Path
				if !strings.HasPrefix(r.Out.URL.Path, "/") {
					r.Out.URL.Path = "/" + r.Out.URL.Path
				}
			}

			// r.SetXForwarded()
		},
		ModifyResponse: func(resp *http.Response) error {
			location := resp.Header.Get("Location")
			if location == "" {
				return nil
			}

			ctxReq := request.Extract(resp.Request.Context())
			if ctxReq == nil {
				return nil
			}

			resp.Header.Set("Location", rewriteRedirectLocation(location, ctxReq.App.PathPrefix))

			return nil
		},
		// ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
		// },
	}

	return proxy
}

func rewriteRedirectLocation(location, pathPrefix string) string {
	if pathPrefix == "" {
		return location
	}

	// Keep absolute and host-relative redirects untouched.
	if strings.Contains(location, "://") || strings.HasPrefix(location, "//") {
		return location
	}
	if !strings.HasPrefix(location, "/") {
		return location
	}

	return pathPrefix + location
}
