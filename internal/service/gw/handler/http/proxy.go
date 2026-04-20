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
	app *config.App,
) http.Handler {
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
			r.SetURL(app.Backend.Url)
			// r.SetXForwarded()
		},
		// ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
		// },
	}

	return proxy
}
