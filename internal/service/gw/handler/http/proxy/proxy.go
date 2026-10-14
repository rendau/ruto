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
	"net/url"
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
			setXForwarded(r)

			r.Out.URL.Path = backendUrl.JoinPath(customPath).Path
			if !strings.HasPrefix(r.Out.URL.Path, "/") {
				r.Out.URL.Path = "/" + r.Out.URL.Path
			}
		}
	} else {
		rewriteFunc = func(r *httputil.ProxyRequest) {
			r.SetURL(backendUrl)
			setXForwarded(r)
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
			resp.Header.Set("Location", rewriteLocation(location, backendUrl, app.PathPrefix))
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

// setXForwarded passes the client IP on like nginx's $proxy_add_x_forwarded_for:
// the inbound X-Forwarded-For (stripped by Rewrite) gets the client IP appended.
// TLS is terminated by the edge proxy in front of ruto, so its X-Forwarded-Proto
// and X-Forwarded-Host win over the plain-http values SetXForwarded derives.
func setXForwarded(r *httputil.ProxyRequest) {
	r.Out.Header["X-Forwarded-For"] = r.In.Header["X-Forwarded-For"]
	r.SetXForwarded()

	for _, name := range []string{"X-Forwarded-Proto", "X-Forwarded-Host"} {
		if values := r.In.Header.Values(name); len(values) > 0 {
			r.Out.Header[name] = values
		}
	}
}

// rewriteLocation maps a backend redirect onto the gateway, like nginx's
// "proxy_redirect default": the backend URL path is replaced with the app
// prefix. A Location outside the backend path, a path-relative one and one
// pointing to another host is returned untouched. An absolute Location on the
// backend host becomes host-relative, since that host is internal.
func rewriteLocation(location string, backendUrl *url.URL, prefix string) string {
	ref := location

	if !strings.HasPrefix(location, "/") || strings.HasPrefix(location, "//") {
		u, err := url.Parse(location)
		if err != nil || u.Host == "" {
			return location
		}
		if (u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https") ||
			!strings.EqualFold(u.Host, backendUrl.Host) {
			return location
		}

		// cut the original string instead of re-encoding u to keep escaping as is
		ref = location[strings.Index(location, "//")+2:]
		if i := strings.IndexAny(ref, "/?#"); i >= 0 {
			ref = ref[i:]
		} else {
			ref = ""
		}
		if !strings.HasPrefix(ref, "/") {
			ref = "/" + ref
		}
	}

	backendPath := strings.TrimRight(backendUrl.EscapedPath(), "/")
	if backendPath == "" {
		return prefix + ref
	}

	rest, ok := strings.CutPrefix(ref, backendPath)
	if !ok || (rest != "" && !strings.ContainsAny(rest[:1], "/?#")) {
		return location
	}

	return prefix + rest
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
