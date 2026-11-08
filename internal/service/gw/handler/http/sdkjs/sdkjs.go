// Package sdkjs serves the browser bundle of the JS SDK (sdk/js,
// dist/ruto-auth.iife.js) from the gateway, so a page without a bundler takes
// it from the host it talks to anyway and always gets the build that matches
// the running gateway.
//
// The file is read once at start; the gzip and brotli copies are made then
// too, at the best level, since nothing has to be compressed per request.
package sdkjs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/go-chi/chi/v5"
	"github.com/klauspost/compress/gzip"

	"github.com/rendau/ruto/internal/constant"
	"github.com/rendau/ruto/internal/infra/acceptenc"
)

// Path is where every gateway serves the bundle; it lives under the reserved
// /_auth prefix, next to the login routes the bundle talks to.
const Path = "/_auth/ruto-auth.js"

// cacheControl: the browser (and a CDN in front) may keep the file for five
// minutes, and use a stale copy for a day while it fetches a fresh one in the
// background, so a page never waits for the script after the first visit, and
// a new gateway build reaches the fronts within minutes. The ETag makes the
// revalidation a 304 most of the time.
const cacheControl = "public, max-age=300, stale-while-revalidate=86400"

// candidates are tried in order when no file is configured: the docker image,
// then a checkout with the SDK built.
var candidates = []string{
	"./sdk-dist/ruto-auth.iife.js",
	"./sdk/js/dist/ruto-auth.iife.js",
}

// offered is the server's preference order among the compressed copies.
var offered = []string{constant.CompressionAlgorithmBr, constant.CompressionAlgorithmGzip}

var current atomic.Pointer[Handler]

// Set installs the handler every router built from now on serves; nil makes
// the route answer 404.
func Set(h *Handler) {
	current.Store(h)
}

// Register adds the bundle route to the router. The route exists whether or
// not a bundle was loaded, so a front gets a clear 404 instead of a proxy
// error from some app.
func Register(router chi.Router) {
	serve := func(w http.ResponseWriter, r *http.Request) {
		h := current.Load()
		if h == nil {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	}
	router.Get(Path, serve)
	router.Head(Path, serve)
}

// Load reads the bundle from path, or from the first candidate that exists
// when path is empty. A configured path that cannot be read is an error; with
// no path and no candidate present the error wraps fs.ErrNotExist.
func Load(path string) (*Handler, error) {
	if path != "" {
		return load(path)
	}

	for _, candidate := range candidates {
		h, err := load(candidate)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return h, err
	}

	return nil, fmt.Errorf("none of %v: %w", candidates, fs.ErrNotExist)
}

func load(path string) (*Handler, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("os.ReadFile: %w", err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("os.Stat: %w", err)
	}

	h, err := New(body, stat.ModTime())
	if err != nil {
		return nil, fmt.Errorf("New(%s): %w", path, err)
	}

	return h, nil
}

type variant struct {
	body []byte
	etag string
}

// Handler serves one build of the bundle in the coding the client accepts.
type Handler struct {
	modTime  time.Time
	identity variant
	encoded  map[string]variant
}

// New prepares the variants of body. modTime is the build time the browser
// gets as Last-Modified.
func New(body []byte, modTime time.Time) (*Handler, error) {
	sum := sha256.Sum256(body)
	tag := hex.EncodeToString(sum[:16])

	h := &Handler{
		modTime:  modTime,
		identity: variant{body: body, etag: `"` + tag + `"`},
		encoded:  map[string]variant{},
	}

	// a strong ETag per coding: the bytes differ, so a cache must not hand a
	// brotli body to a client that asked for gzip
	for _, coding := range offered {
		encoded, err := encode(coding, body)
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", coding, err)
		}
		h.encoded[coding] = variant{body: encoded, etag: `"` + tag + "-" + coding + `"`}
	}

	return h, nil
}

func encode(coding string, body []byte) ([]byte, error) {
	var buf bytes.Buffer

	switch coding {
	case constant.CompressionAlgorithmBr:
		w := brotli.NewWriterLevel(&buf, brotli.BestCompression)
		if _, err := w.Write(body); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
	case constant.CompressionAlgorithmGzip:
		w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		if err != nil {
			return nil, err
		}
		if _, err = w.Write(body); err != nil {
			return nil, err
		}
		if err = w.Close(); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported coding %q", coding)
	}

	return buf.Bytes(), nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	v := h.identity
	coding := acceptenc.Pick(r.Header.Get("Accept-Encoding"), offered)
	if coding != "" {
		v = h.encoded[coding]
		w.Header().Set("Content-Encoding", coding)
	}

	header := w.Header()
	header.Add("Vary", "Accept-Encoding")
	header.Set("Content-Type", "text/javascript; charset=utf-8")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Cache-Control", cacheControl)
	header.Set("ETag", v.etag)
	// ServeContent leaves Content-Length out of an encoded response (it cannot
	// know the bytes are already the final form); here they are
	header.Set("Content-Length", strconv.Itoa(len(v.body)))

	// ServeContent does the conditional requests (If-None-Match → 304,
	// If-Modified-Since) and HEAD; the name is empty so it keeps the
	// Content-Type set above instead of guessing one.
	http.ServeContent(w, r, "", h.modTime, bytes.NewReader(v.body))
}
