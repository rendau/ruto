package sdkjs

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/go-chi/chi/v5"
	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bundle = []byte("var RutoAuth=(()=>{" + string(bytes.Repeat([]byte("x=1;"), 2000)) + "})();")

func newHandler(t *testing.T) *Handler {
	t.Helper()
	h, err := New(bundle, time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	return h
}

func get(h http.Handler, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, Path, nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestServesTheBundleInTheAcceptedCoding(t *testing.T) {
	h := newHandler(t)

	t.Run("identity", func(t *testing.T) {
		w := get(h, nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, bundle, w.Body.Bytes())
		assert.Empty(t, w.Header().Get("Content-Encoding"))
		assert.Equal(t, "text/javascript; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Equal(t, cacheControl, w.Header().Get("Cache-Control"))
		assert.Equal(t, "Accept-Encoding", w.Header().Get("Vary"))
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, strconv.Itoa(len(bundle)), w.Header().Get("Content-Length"))
		assert.Equal(t, "Sun, 08 Nov 2026 12:00:00 GMT", w.Header().Get("Last-Modified"))
		assert.Regexp(t, `^"[0-9a-f]{32}"$`, w.Header().Get("ETag"))
	})

	t.Run("brotli preferred over gzip", func(t *testing.T) {
		w := get(h, map[string]string{"Accept-Encoding": "gzip, deflate, br, zstd"})
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "br", w.Header().Get("Content-Encoding"))
		assert.Less(t, w.Body.Len(), len(bundle)/4)
		assert.Equal(t, strconv.Itoa(w.Body.Len()), w.Header().Get("Content-Length"))
		decoded, err := io.ReadAll(brotli.NewReader(w.Body))
		require.NoError(t, err)
		assert.Equal(t, bundle, decoded)
		assert.Regexp(t, `^"[0-9a-f]{32}-br"$`, w.Header().Get("ETag"))
	})

	t.Run("gzip", func(t *testing.T) {
		w := get(h, map[string]string{"Accept-Encoding": "gzip"})
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
		zr, err := gzip.NewReader(w.Body)
		require.NoError(t, err)
		decoded, err := io.ReadAll(zr)
		require.NoError(t, err)
		assert.Equal(t, bundle, decoded)
		assert.Regexp(t, `^"[0-9a-f]{32}-gzip"$`, w.Header().Get("ETag"))
	})

	t.Run("refused codings fall back to identity", func(t *testing.T) {
		w := get(h, map[string]string{"Accept-Encoding": "br;q=0, gzip;q=0"})
		require.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Header().Get("Content-Encoding"))
		assert.Equal(t, bundle, w.Body.Bytes())
	})
}

func TestRevalidation(t *testing.T) {
	h := newHandler(t)

	first := get(h, map[string]string{"Accept-Encoding": "br"})
	etag := first.Header().Get("ETag")

	w := get(h, map[string]string{"Accept-Encoding": "br", "If-None-Match": etag})
	assert.Equal(t, http.StatusNotModified, w.Code)
	assert.Empty(t, w.Body.Bytes())
	assert.Equal(t, etag, w.Header().Get("ETag"))
	assert.Equal(t, cacheControl, w.Header().Get("Cache-Control"))

	// the gzip copy has another tag: a cache keyed badly must not get a 304
	w = get(h, map[string]string{"Accept-Encoding": "gzip", "If-None-Match": etag})
	assert.Equal(t, http.StatusOK, w.Code)

	w = get(h, map[string]string{"If-Modified-Since": "Sun, 08 Nov 2026 12:00:00 GMT"})
	assert.Equal(t, http.StatusNotModified, w.Code)

	w = get(h, map[string]string{"If-Modified-Since": "Sat, 07 Nov 2026 12:00:00 GMT"})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHead(t *testing.T) {
	h := newHandler(t)
	r := httptest.NewRequest(http.MethodHead, Path, nil)
	r.Header.Set("Accept-Encoding", "br")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.Bytes())
	assert.Equal(t, "br", w.Header().Get("Content-Encoding"))
	assert.NotEmpty(t, w.Header().Get("Content-Length"))
}

func TestRegister(t *testing.T) {
	Set(nil)
	t.Cleanup(func() { Set(nil) })

	router := chi.NewRouter()
	Register(router)

	w := get(router, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "no bundle loaded: a plain 404")

	Set(newHandler(t))
	w = get(router, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, bundle, w.Body.Bytes())

	r := httptest.NewRequest(http.MethodPost, Path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "ruto-auth.iife.js")
	require.NoError(t, os.WriteFile(file, bundle, 0o644))

	h, err := Load(file)
	require.NoError(t, err)
	assert.Equal(t, bundle, get(h, nil).Body.Bytes())

	_, err = Load(filepath.Join(dir, "missing.js"))
	require.Error(t, err)
	assert.ErrorIs(t, err, fs.ErrNotExist)

	// no file configured, and no candidate in the test's working directory
	_, err = Load("")
	assert.ErrorIs(t, err, fs.ErrNotExist)
}
