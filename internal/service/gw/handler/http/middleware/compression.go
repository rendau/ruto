package middleware

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"

	"github.com/samber/lo"

	compressionModel "github.com/rendau/ruto/internal/domain/compression/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/infra/acceptenc"
	"github.com/rendau/ruto/internal/service/gw/service/compress"
)

// NewCompression compresses the response body with the first coding of the
// endpoint's list the client accepts, and, in "identity" backend mode, strips
// Accept-Encoding from the request so the backend answers uncompressed. It is
// the outermost middleware: the log, the cache and the response script all
// see the plain body, and the cache keeps one copy for every client.
func NewCompression(ep *endpointModel.Endpoint) Middleware {
	conf := ep.Compression
	identity := conf.BackendIdentity()
	offered := lo.Filter(conf.AlgorithmsOrDefault(), func(coding string, _ int) bool {
		return compress.Supported(coding)
	})
	if !conf.ResponseEnabled() {
		offered = nil
	}
	minSize := conf.MinSizeOrDefault()

	if !identity && len(offered) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			acceptEncoding := r.Header.Get("Accept-Encoding")
			if identity {
				r.Header.Del("Accept-Encoding")
			}

			coding := ""
			if r.Method != http.MethodHead {
				coding = acceptenc.Pick(acceptEncoding, offered)
			}
			if coding == "" {
				next.ServeHTTP(w, r)
				return
			}

			cw := &compressWriter{ResponseWriter: w, coding: coding, conf: &conf, minSize: minSize}
			next.ServeHTTP(cw, r)
			cw.finish()
		})
	}
}

type compressState int

const (
	// stateUndecided holds the body back until its size is known: the
	// Content-Length, enough bytes, or a flush / the end of the response.
	stateUndecided compressState = iota
	statePlain
	stateCompressed
)

// compressWriter decides per response whether to compress it, right when its
// headers go out, and from then on either passes bytes through or runs them
// through the encoder.
type compressWriter struct {
	http.ResponseWriter
	coding  string
	conf    *compressionModel.Compression
	minSize int

	status int
	state  compressState
	held   bytes.Buffer
	enc    compress.Encoder
}

func (c *compressWriter) WriteHeader(status int) {
	if c.status != 0 {
		return
	}
	// Informational responses go out as they are; the final status is still to
	// come. 101 ends the HTTP exchange: the connection is handed over.
	if status < http.StatusOK && status != http.StatusSwitchingProtocols {
		c.ResponseWriter.WriteHeader(status)
		return
	}

	c.status = status
	if !c.eligible() {
		c.startPlain()
		return
	}

	if cl := c.Header().Get("Content-Length"); cl != "" {
		if n, err := strconv.Atoi(cl); err == nil && n < c.minSize {
			c.startPlain()
			return
		}
		c.startCompressed()
	}
}

// eligible checks the headers the backend sent: a body that is already
// encoded, partial, marked no-transform, empty by status or not of a listed
// media type goes out as is.
func (c *compressWriter) eligible() bool {
	switch c.status {
	case http.StatusSwitchingProtocols, http.StatusNoContent, http.StatusResetContent,
		http.StatusPartialContent, http.StatusNotModified:
		return false
	}

	h := c.Header()
	if enc := h.Get("Content-Encoding"); enc != "" && !strings.EqualFold(enc, "identity") {
		return false
	}
	if h.Get("Content-Range") != "" {
		return false
	}
	if strings.Contains(strings.ToLower(strings.Join(h.Values("Cache-Control"), ",")), "no-transform") {
		return false
	}

	return c.conf.MatchesContentType(h.Get("Content-Type"))
}

func (c *compressWriter) startPlain() {
	c.state = statePlain
	c.ResponseWriter.WriteHeader(c.status)
	c.releaseHeld(c.ResponseWriter)
}

func (c *compressWriter) startCompressed() {
	h := c.Header()
	h.Set("Content-Encoding", c.coding)
	if !strings.Contains(strings.ToLower(strings.Join(h.Values("Vary"), ",")), "accept-encoding") {
		h.Add("Vary", "Accept-Encoding")
	}
	h.Del("Content-Length")
	// Byte ranges would address the compressed stream, which nobody asked for.
	h.Del("Accept-Ranges")
	if etag := h.Get("ETag"); etag != "" && !strings.HasPrefix(etag, "W/") {
		h.Set("ETag", "W/"+etag)
	}

	c.state = stateCompressed
	c.ResponseWriter.WriteHeader(c.status)
	c.enc, _ = compress.NewEncoder(c.coding, c.ResponseWriter)
	c.releaseHeld(c.enc)
}

func (c *compressWriter) releaseHeld(w interface{ Write([]byte) (int, error) }) {
	if c.held.Len() == 0 {
		return
	}
	_, _ = w.Write(c.held.Bytes())
	c.held = bytes.Buffer{}
}

func (c *compressWriter) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.WriteHeader(http.StatusOK)
	}

	switch c.state {
	case statePlain:
		return c.ResponseWriter.Write(p)
	case stateCompressed:
		return c.enc.Write(p)
	}

	c.held.Write(p)
	if c.held.Len() >= c.minSize {
		c.startCompressed()
	}
	return len(p), nil
}

// Flush pushes everything through to the client. A response still undecided
// when its first bytes are flushed is streamed, and its size unknown: it goes
// out as is. A flush with no body yet (the proxy flushes right after the
// headers of a chunked response) is held back, so the body still decides.
func (c *compressWriter) Flush() {
	switch c.state {
	case stateUndecided:
		if c.status == 0 || c.held.Len() == 0 {
			return
		}
		c.startPlain()
	case stateCompressed:
		_ = c.enc.Flush()
	}
	_ = http.NewResponseController(c.ResponseWriter).Flush()
}

// finish is called once the handler returned: a body held back because it
// stayed under min_size goes out as is, an encoder closes its stream.
func (c *compressWriter) finish() {
	switch c.state {
	case stateUndecided:
		// Nothing written at all: net/http answers 200 itself.
		if c.status != 0 {
			c.startPlain()
		}
	case stateCompressed:
		_ = c.enc.Close()
	}
}

func (c *compressWriter) Unwrap() http.ResponseWriter {
	return c.ResponseWriter
}
