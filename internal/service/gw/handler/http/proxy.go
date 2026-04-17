package http

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/rendau/ruto/internal/model/config"
)

func newProxy(
	root *config.Root,
	app *config.App,
) http.Handler {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   2 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		DisableCompression:  true,
		TLSHandshakeTimeout: 2 * time.Second,
		MaxIdleConnsPerHost: 50,
	}

	if root.Timeout.ReadHeader > 0 {
		transport.ResponseHeaderTimeout = root.Timeout.ReadHeader
	}

	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(app.Backend.Url)
			r.SetXForwarded()
			// r.Out.Host = conf.TargetHost
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
