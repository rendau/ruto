package http

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/rendau/ruto/internal/model/config"
)

func newProxy(
	root *config.Root,
	app *config.App,
	endpoint *config.Endpoint,
	targetUrl *url.URL,
) http.Handler {
	targetScheme := targetBaseURL.Scheme
	targetHost := targetBaseURL.Host

	proxy := &httputil.ReverseProxy{
		// Director: func(req *http.Request) {
		// 	req.URL.Scheme = targetScheme
		// 	req.URL.Host = targetHost
		// 	req.URL.Path = backendPath
		// 	req.URL.RawPath = ""
		// 	req.Host = targetHost
		// },
		// ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
		// 	slog.Error("reverse proxy request failed",
		// 		"error", err,
		// 		"endpoint_id", endpointID,
		// 		"method", r.Method,
		// 		"path", r.URL.Path,
		// 	)
		// 	http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		// },
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext: (&net.Dialer{
				Timeout:   2 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			DisableCompression:  true,
			TLSHandshakeTimeout: 2 * time.Second,
			// ResponseHeaderTimeout: conf.TargetTimeout,
			MaxIdleConnsPerHost: 20,
		},
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(conf.TargetUrl)
			if len(conf.TargetHeaders) > 0 {
				for k, v := range conf.TargetHeaders {
					r.Out.Header.Add(k, v)
				}
			}
			r.SetXForwarded()
			if conf.TargetHost != "" {
				r.Out.Host = conf.TargetHost
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if r.Context().Err() != nil {
				return
			}
			w.WriteHeader(http.StatusBadGateway)
		},
	}

	return proxy
}
