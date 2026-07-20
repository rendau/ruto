package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"syscall"
	"time"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxyerr"
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
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if r.Context().Err() != nil {
				return
			}
			reason := classifyProxyError(err)
			proxyerr.Set(r.Context(), reason)
			slog.Error("proxy error "+r.Method+" "+r.URL.Path,
				"reason", reason,
				"error", err.Error(),
				"app_name", app.Name,
			)
			w.WriteHeader(http.StatusBadGateway)
		},
	}

	return proxy
}

// classifyProxyError turns a transport-level proxy error into a short,
// human-readable cause for the logs (e.g. "backend closed connection")
// instead of a bare 502 / "status code error".
func classifyProxyError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return "client canceled request"
	case errors.Is(err, context.DeadlineExceeded):
		return "backend response timeout"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return "backend closed connection"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "backend connection refused"
	case errors.Is(err, syscall.ECONNRESET):
		return "backend reset connection"
	}

	if _, ok := errors.AsType[*net.DNSError](err); ok {
		return "backend host not resolved"
	}
	if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
		return "backend connect timeout"
	}

	return "backend request failed"
}
