package http

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	compressionModel "github.com/rendau/ruto/internal/domain/compression/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
	"github.com/rendau/ruto/internal/service/gw/service/cache"
)

var bigJSON = strings.Repeat(`{"id":1,"name":"item","tags":["a","b"]},`, 100)

// newCompressionTestService proxies /app/* to backend; the last Accept-Encoding
// the backend got is kept for the test.
func newCompressionTestService(t *testing.T, conf compressionModel.Compression, backend http.HandlerFunc) (*Service, *atomic.Value) {
	t.Helper()

	var seenAcceptEncoding atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAcceptEncoding.Store(r.Header.Get("Accept-Encoding"))
		backend(w, r)
	}))
	t.Cleanup(server.Close)

	ep := &endpointModel.Endpoint{
		Id:     "ep1",
		Active: true,
		Type:   endpointModel.TypeHTTP,
		Http:   endpointModel.Http{Method: "*", Path: "*"},
		Cache:  endpointModel.Cache{Server: endpointModel.CacheServer{Enabled: true, Ttl: 60}},
	}
	snapshot := rootModel.NewEmpty()
	snapshot.Compression = conf
	snapshot.Apps = []*appModel.App{{
		Id:            "app1",
		Active:        true,
		PathPrefix:    "/app",
		Name:          "app",
		AllowWildcard: true,
		Backend:       appModel.Backend{Url: server.URL},
		Endpoints:     []*endpointModel.Endpoint{ep},
	}}
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()
	snapshot.Interpolate()

	service, err := New(snapshot, proxy.NewTransport(0), cache.New(1<<20))
	require.NoError(t, err)

	return service, &seenAcceptEncoding
}

func jsonBackend(body string, header http.Header) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		for k, vs := range header {
			w.Header()[k] = vs
		}
		_, _ = io.WriteString(w, body)
	}
}

func decodeBody(t *testing.T, coding string, b []byte) string {
	t.Helper()

	var r io.Reader
	switch coding {
	case "br":
		r = brotli.NewReader(bytes.NewReader(b))
	case "gzip":
		zr, err := gzip.NewReader(bytes.NewReader(b))
		require.NoError(t, err)
		r = zr
	case "zstd":
		zr, err := zstd.NewReader(bytes.NewReader(b))
		require.NoError(t, err)
		defer zr.Close()
		r = zr
	case "":
		r = bytes.NewReader(b)
	}
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func acceptHeader(v string) http.Header {
	return http.Header{"Accept-Encoding": {v}}
}

func TestCompression_PicksByPreference(t *testing.T) {
	service, seen := newCompressionTestService(t, compressionModel.Default(), jsonBackend(bigJSON, nil))

	tests := []struct {
		accept string
		want   string
	}{
		{accept: "gzip, deflate, br, zstd", want: "br"},
		{accept: "gzip, zstd", want: "zstd"},
		{accept: "gzip", want: "gzip"},
		{accept: "br;q=0, gzip", want: "gzip"},
		{accept: "deflate", want: ""},
		{accept: "", want: ""},
	}
	for i, tt := range tests {
		t.Run(tt.accept, func(t *testing.T) {
			// A distinct query per case keeps the server cache out of the way.
			rec := performRequest(service, http.MethodGet, "/app/items?case="+strconv.Itoa(i), acceptHeader(tt.accept))

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.want, rec.Header().Get("Content-Encoding"))
			assert.Equal(t, bigJSON, decodeBody(t, tt.want, rec.Body.Bytes()))
			assert.Empty(t, seen.Load(), "identity mode must not pass Accept-Encoding to the backend")
			if tt.want != "" {
				assert.Less(t, rec.Body.Len(), len(bigJSON)/4)
				assert.Equal(t, "Accept-Encoding", rec.Header().Get("Vary"))
				assert.Empty(t, rec.Header().Get("Content-Length"))
			}
		})
	}
}

func TestCompression_SkipsSmallAndUnlistedBodies(t *testing.T) {
	t.Run("under min_size", func(t *testing.T) {
		service, _ := newCompressionTestService(t, compressionModel.Default(), jsonBackend(`{"ok":true}`, nil))
		rec := performRequest(service, http.MethodGet, "/app/items", acceptHeader("br"))

		assert.Empty(t, rec.Header().Get("Content-Encoding"))
		assert.Equal(t, `{"ok":true}`, rec.Body.String())
	})

	t.Run("custom min_size", func(t *testing.T) {
		conf := compressionModel.Default()
		conf.MinSize = 8
		service, _ := newCompressionTestService(t, conf, jsonBackend(`{"ok":true}`, nil))
		rec := performRequest(service, http.MethodGet, "/app/items", acceptHeader("br"))

		assert.Equal(t, "br", rec.Header().Get("Content-Encoding"))
		assert.Equal(t, `{"ok":true}`, decodeBody(t, "br", rec.Body.Bytes()))
	})

	t.Run("binary type", func(t *testing.T) {
		service, _ := newCompressionTestService(t, compressionModel.Default(), jsonBackend(bigJSON, http.Header{"Content-Type": {"image/png"}}))
		rec := performRequest(service, http.MethodGet, "/app/items", acceptHeader("br"))

		assert.Empty(t, rec.Header().Get("Content-Encoding"))
		assert.Equal(t, bigJSON, rec.Body.String())
	})

	t.Run("no-transform", func(t *testing.T) {
		service, _ := newCompressionTestService(t, compressionModel.Default(), jsonBackend(bigJSON, http.Header{"Cache-Control": {"no-transform"}}))
		rec := performRequest(service, http.MethodGet, "/app/items", acceptHeader("br"))

		assert.Empty(t, rec.Header().Get("Content-Encoding"))
	})

	t.Run("head", func(t *testing.T) {
		service, _ := newCompressionTestService(t, compressionModel.Default(), jsonBackend(bigJSON, nil))
		rec := performRequest(service, http.MethodHead, "/app/items", acceptHeader("br"))

		assert.Empty(t, rec.Header().Get("Content-Encoding"))
	})

	t.Run("response off", func(t *testing.T) {
		conf := compressionModel.Default()
		conf.Response = "off"
		service, seen := newCompressionTestService(t, conf, jsonBackend(bigJSON, nil))
		rec := performRequest(service, http.MethodGet, "/app/items", acceptHeader("br"))

		assert.Empty(t, rec.Header().Get("Content-Encoding"))
		assert.Equal(t, bigJSON, rec.Body.String())
		assert.Empty(t, seen.Load(), "backend still asked for a plain body")
	})
}

func TestCompression_HeadersOfCompressedResponse(t *testing.T) {
	service, _ := newCompressionTestService(t, compressionModel.Default(), jsonBackend(bigJSON, http.Header{
		"ETag":          {`"abc"`},
		"Accept-Ranges": {"bytes"},
		"Vary":          {"Origin"},
	}))
	rec := performRequest(service, http.MethodGet, "/app/items", acceptHeader("gzip"))

	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Equal(t, `W/"abc"`, rec.Header().Get("ETag"))
	assert.Empty(t, rec.Header().Get("Accept-Ranges"))
	assert.Equal(t, []string{"Origin", "Accept-Encoding"}, rec.Header().Values("Vary"))
}

func TestCompression_ClientModePassesBackendEncodingThrough(t *testing.T) {
	conf := compressionModel.Default()
	conf.Backend = "client"

	var gzipped bytes.Buffer
	zw := gzip.NewWriter(&gzipped)
	_, _ = io.WriteString(zw, bigJSON)
	_ = zw.Close()

	service, seen := newCompressionTestService(t, conf, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			_, _ = w.Write(gzipped.Bytes())
			return
		}
		_, _ = io.WriteString(w, bigJSON)
	})

	rec := performRequest(service, http.MethodGet, "/app/items", acceptHeader("br, gzip"))
	assert.Equal(t, "br, gzip", seen.Load())
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"), "the backend's own encoding is kept, not re-encoded")
	assert.Equal(t, bigJSON, decodeBody(t, "gzip", rec.Body.Bytes()))

	rec = performRequest(service, http.MethodGet, "/app/items", acceptHeader("br"))
	assert.Equal(t, "br", rec.Header().Get("Content-Encoding"), "a plain backend answer is compressed by the gateway")
	assert.Equal(t, bigJSON, decodeBody(t, "br", rec.Body.Bytes()))
}

func TestCompression_CacheKeepsOnePlainCopy(t *testing.T) {
	var hits atomic.Int64
	service, _ := newCompressionTestService(t, compressionModel.Default(), func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		jsonBackend(bigJSON, nil)(w, r)
	})

	rec := performRequest(service, http.MethodGet, "/app/items", acceptHeader("br"))
	assert.Equal(t, "MISS", rec.Header().Get("X-Cache"))
	assert.Equal(t, "br", rec.Header().Get("Content-Encoding"))

	rec = performRequest(service, http.MethodGet, "/app/items", acceptHeader("gzip"))
	assert.Equal(t, "HIT", rec.Header().Get("X-Cache"), "the plain copy serves every encoding")
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Equal(t, bigJSON, decodeBody(t, "gzip", rec.Body.Bytes()))

	rec = performRequest(service, http.MethodGet, "/app/items", nil)
	assert.Equal(t, "HIT", rec.Header().Get("X-Cache"))
	assert.Empty(t, rec.Header().Get("Content-Encoding"))
	assert.Equal(t, bigJSON, rec.Body.String())

	assert.Equal(t, int64(1), hits.Load())
}

func TestCompression_ChunkedBigBodyIsCompressed(t *testing.T) {
	service, _ := newCompressionTestService(t, compressionModel.Default(), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Written in pieces so the backend answers chunked, without a length.
		for i := 0; i < len(bigJSON); i += 1500 {
			_, _ = io.WriteString(w, bigJSON[i:min(i+1500, len(bigJSON))])
			http.NewResponseController(w).Flush()
		}
	})

	rec := performRequest(service, http.MethodGet, "/app/items", acceptHeader("zstd"))

	assert.Equal(t, "zstd", rec.Header().Get("Content-Encoding"))
	assert.Equal(t, bigJSON, decodeBody(t, "zstd", rec.Body.Bytes()))
}

// A small body without Content-Length is held until it ends and goes out
// with its size: the proxy's per-piece flushes do not commit it to chunked
// transfer, so the edge proxy does not compress it either.
func TestCompression_SmallChunkedBodyGetsContentLength(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "404 json", status: http.StatusNotFound, body: `{"error":"not found"}`},
		{name: "302 html", status: http.StatusFound, body: strings.Repeat("r", 330)},
		{name: "200 under threshold", status: http.StatusOK, body: strings.Repeat("z", 900)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _ := newCompressionTestService(t, compressionModel.Default(), func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Transfer-Encoding", "chunked")
				w.WriteHeader(tt.status)
				cut := min(10, len(tt.body))
				_, _ = io.WriteString(w, tt.body[:cut])
				http.NewResponseController(w).Flush()
				_, _ = io.WriteString(w, tt.body[cut:])
			})

			rec := performRequest(service, http.MethodGet, "/app/items", acceptHeader("gzip"))

			assert.Equal(t, tt.status, rec.Code)
			assert.Empty(t, rec.Header().Get("Content-Encoding"))
			assert.Equal(t, strconv.Itoa(len(tt.body)), rec.Header().Get("Content-Length"))
			assert.Equal(t, tt.body, rec.Body.String())
			assert.False(t, rec.Flushed, "a held response must not be flushed piecewise")
		})
	}
}

// A chunked body that crosses the threshold after a small first piece is
// still compressed: the early flush does not commit it to plain transfer.
func TestCompression_SmallFirstPieceThenBigBodyIsCompressed(t *testing.T) {
	service, _ := newCompressionTestService(t, compressionModel.Default(), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, bigJSON[:100])
		http.NewResponseController(w).Flush()
		_, _ = io.WriteString(w, bigJSON[100:])
	})

	rec := performRequest(service, http.MethodGet, "/app/items", acceptHeader("gzip"))

	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Equal(t, bigJSON, decodeBody(t, "gzip", rec.Body.Bytes()))
}

// A stream whose first flush carries less than min_size goes out as is, and
// each event reaches the client right away.
func TestCompression_StreamFlushesThrough(t *testing.T) {
	release := make(chan struct{})
	service, _ := newCompressionTestService(t, compressionModel.Default(), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: one\n\n")
		http.NewResponseController(w).Flush()
		<-release
		_, _ = io.WriteString(w, "data: two\n\n")
	})

	gw := httptest.NewServer(service)
	t.Cleanup(gw.Close)

	req, _ := http.NewRequest(http.MethodGet, gw.URL+"/app/events", nil)
	req.Header.Set("Accept-Encoding", "br")
	resp, err := gw.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Empty(t, resp.Header.Get("Content-Encoding"))

	type lineResult struct {
		line string
		err  error
	}
	lines := make(chan lineResult, 4)
	go func() {
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			lines <- lineResult{line: line, err: err}
			if err != nil {
				return
			}
		}
	}()

	select {
	case got := <-lines:
		require.NoError(t, got.err)
		assert.Equal(t, "data: one\n", got.line)
	case <-time.After(2 * time.Second):
		t.Fatal("first event did not reach the client before the backend wrote the second")
	}
	close(release)
}
