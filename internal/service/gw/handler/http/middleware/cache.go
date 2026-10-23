package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/service/cache"
)

const cacheStatusHeader = "X-Cache"

// NewServerCache answers GET requests from the gateway cache and stores
// cacheable 200 responses on a miss. Concurrent misses of one key all go to
// the backend; the last stored response wins. Responses are streamed to the
// client while they are copied, so a miss costs no extra latency.
func NewServerCache(app *appModel.App, ep *endpointModel.Endpoint, store *cache.Service) Middleware {
	conf := ep.Cache.Server
	if !conf.Enabled || store == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	keyHeaders := cacheKeyHeaders(ep)
	keyPrefix := cacheKeyPrefix(app, ep)
	ttl := time.Duration(conf.Ttl) * time.Second
	maxBodySize := conf.MaxBodySizeOrDefault()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}

			key := cacheKey(keyPrefix, r, keyHeaders)

			if entry := store.Get(key); entry != nil {
				writeCachedResponse(w, entry)
				return
			}

			capt := &cacheCapture{ResponseWriter: w, limit: maxBodySize}
			next.ServeHTTP(capt, r)

			if r.Context().Err() == nil && capt.cacheable() {
				store.Set(key, &cache.Entry{
					Status: capt.status,
					Header: capt.header,
					Body:   capt.body.Bytes(),
				}, ttl)
			}
		})
	}
}

// NewBrowserCache sets Cache-Control on successful GET/HEAD responses so the
// client keeps them.
func NewBrowserCache(ep *endpointModel.Endpoint) Middleware {
	conf := ep.Cache.Browser
	if !conf.Enabled {
		return func(next http.Handler) http.Handler { return next }
	}

	value := conf.HeaderValue()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(&cacheControlWriter{ResponseWriter: w, value: value, override: conf.Override}, r)
		})
	}
}

// cacheKeyHeaders lists request headers that split the cache: Accept-Encoding
// (the backend may answer compressed), the configured ones and, with auth on,
// the credentials — so one user never gets a response cached for another,
// unless the endpoint declares its cache shared.
func cacheKeyHeaders(ep *endpointModel.Endpoint) []string {
	result := []string{"Accept-Encoding"}
	result = append(result, ep.Cache.Server.VaryHeaders...)

	if ep.Auth.Enabled && !ep.Cache.Server.Shared {
		result = append(result, "Authorization")
		for _, method := range ep.Auth.Methods {
			if method.APIKey == nil {
				continue
			}
			header := strings.TrimSpace(method.APIKey.Header)
			if header == "" {
				header = constant.AuthAPIKeyDefaultHeader
			}
			result = append(result, http.CanonicalHeaderKey(header))
		}
	}

	result = lo.Uniq(result)
	slices.Sort(result)

	return result
}

// cacheKeyPrefix ties cached responses to the endpoint configuration, so any
// change of the endpoint or its app backend starts with an empty cache; old
// entries just expire.
func cacheKeyPrefix(app *appModel.App, ep *endpointModel.Endpoint) string {
	h := sha256.New()
	_ = json.NewEncoder(h).Encode(app.Backend)
	_ = json.NewEncoder(h).Encode(ep)
	return ep.Id + ":" + hex.EncodeToString(h.Sum(nil))[:16]
}

// cacheKey hashes the request parts, keeping credentials out of memory as is.
func cacheKey(prefix string, r *http.Request, headers []string) string {
	h := sha256.New()
	h.Write([]byte(r.URL.Path))
	h.Write([]byte{0})
	h.Write([]byte(r.URL.Query().Encode()))
	for _, name := range headers {
		h.Write([]byte{0})
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write([]byte(strings.Join(r.Header.Values(name), ",")))
	}
	return prefix + ":" + hex.EncodeToString(h.Sum(nil))
}

func writeCachedResponse(w http.ResponseWriter, entry *cache.Entry) {
	dst := w.Header()
	for k, vs := range entry.Header {
		dst[k] = slices.Clone(vs)
	}
	dst.Set(cacheStatusHeader, "HIT")
	dst.Set("Age", strconv.Itoa(int(time.Since(entry.StoredAt).Seconds())))

	w.WriteHeader(entry.Status)
	_, _ = w.Write(entry.Body)
}

// cacheCapture passes the response through to the client and keeps a copy of
// it, up to limit bytes of body, for the cache.
type cacheCapture struct {
	http.ResponseWriter
	limit int

	header      http.Header
	status      int
	body        bytes.Buffer
	wroteHeader bool
	overflow    bool
	writeErr    bool
}

func (c *cacheCapture) WriteHeader(status int) {
	if c.wroteHeader {
		return
	}
	c.wroteHeader = true
	c.status = status

	dst := c.ResponseWriter.Header()
	c.header = dst.Clone()
	dst.Set(cacheStatusHeader, "MISS")

	c.ResponseWriter.WriteHeader(status)
}

func (c *cacheCapture) Write(p []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}

	if !c.overflow {
		if c.body.Len()+len(p) > c.limit {
			c.overflow = true
			c.body = bytes.Buffer{}
		} else {
			c.body.Write(p)
		}
	}

	n, err := c.ResponseWriter.Write(p)
	if err != nil {
		c.writeErr = true
	}
	return n, err
}

func (c *cacheCapture) Unwrap() http.ResponseWriter {
	return c.ResponseWriter
}

// cacheable keeps out partial bodies and responses the backend marked as
// personal or not to be stored: the gateway does not know how a backend tells
// its users apart (e.g. by a cookie), unless the endpoint lists it in
// vary_headers.
func (c *cacheCapture) cacheable() bool {
	if c.status != http.StatusOK || c.overflow || c.writeErr {
		return false
	}
	if cl := c.header.Get("Content-Length"); cl != "" && cl != strconv.Itoa(c.body.Len()) {
		return false
	}
	if c.header.Get("Set-Cookie") != "" {
		return false
	}
	directives := strings.ToLower(strings.Join(c.header.Values("Cache-Control"), ","))
	if strings.Contains(directives, "no-store") || strings.Contains(directives, "private") {
		return false
	}
	return true
}

// cacheControlWriter sets Cache-Control on a 200 response right before its
// headers go out.
type cacheControlWriter struct {
	http.ResponseWriter
	value       string
	override    bool
	wroteHeader bool
}

func (c *cacheControlWriter) WriteHeader(status int) {
	if c.wroteHeader {
		return
	}
	c.wroteHeader = true

	header := c.ResponseWriter.Header()
	if status == http.StatusOK && (c.override || header.Get("Cache-Control") == "") {
		header.Set("Cache-Control", c.value)
		if c.override {
			header.Del("Expires")
			header.Del("Pragma")
		}
	}

	c.ResponseWriter.WriteHeader(status)
}

func (c *cacheControlWriter) Write(p []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	return c.ResponseWriter.Write(p)
}

func (c *cacheControlWriter) Unwrap() http.ResponseWriter {
	return c.ResponseWriter
}
