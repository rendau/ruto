package proxy

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
)

func NewTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout: 2 * time.Second,
		}).DialContext,
		DisableCompression:  true,
		TLSHandshakeTimeout: 2 * time.Second,
		MaxIdleConnsPerHost: 100,
	}
}

func NewProxy(app *appModel.App, customPath string, transport http.RoundTripper) http.Handler {
	backendUrl := app.Backend.ParsedUrl

	var rewriteFunc func(r *httputil.ProxyRequest)

	if customPath != "" {
		rewriteFunc = func(r *httputil.ProxyRequest) {
			r.SetURL(backendUrl)

			r.Out.URL.Path = backendUrl.JoinPath(customPath).Path
			if !strings.HasPrefix(r.Out.URL.Path, "/") {
				r.Out.URL.Path = "/" + r.Out.URL.Path
			}
		}
	} else {
		rewriteFunc = func(r *httputil.ProxyRequest) {
			r.SetURL(backendUrl)
		}
	}

	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite:   rewriteFunc,
		ModifyResponse: func(resp *http.Response) error {
			location := resp.Header.Get("Location")
			if location == "" {
				return nil
			}
			// Keep absolute redirects untouched.
			if strings.Contains(location, "://") || strings.HasPrefix(location, "//") {
				return nil
			}
			// Keep relative redirects untouched.
			if !strings.HasPrefix(location, "/") {
				return nil
			}
			resp.Header.Set("Location", app.PathPrefix+location)
			return nil
		},
	}

	return proxy
}
