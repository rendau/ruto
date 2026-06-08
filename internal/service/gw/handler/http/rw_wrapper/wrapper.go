package rw_wrapper

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

var statusCodeErr = errors.New("status code error")

type Wrapper struct {
	http.ResponseWriter
	statusCode int
	bodyCap    *BodyCapture
	expected   bool
}

func New(w http.ResponseWriter) *Wrapper {
	return &Wrapper{ResponseWriter: w}
}

// CaptureBody enables capturing up to limit bytes of the response body for
// logging.
func (w *Wrapper) CaptureBody(limit int) {
	w.bodyCap = NewBodyCapture(limit)
}

func (w *Wrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *Wrapper) Write(p []byte) (int, error) {
	if w.bodyCap != nil {
		w.bodyCap.Write(p)
	}
	return w.ResponseWriter.Write(p)
}

func (w *Wrapper) GetCapturedBody() string {
	c := w.bodyCap
	if c == nil {
		return ""
	}

	// The proxied bytes are captured as sent downstream, i.e. still compressed
	// when the upstream set Content-Encoding. Decompress so the log is readable.
	decoded, ok := decodeBody(c.buf, w.Header().Get("Content-Encoding"))
	if !ok {
		return c.String()
	}

	truncated := c.truncated
	if len(decoded) > c.limit {
		decoded = decoded[:c.limit]
		truncated = true
	}
	if truncated {
		return string(decoded) + truncatedSuffix
	}
	return string(decoded)
}

// decodeBody decompresses a captured body for logging. The capture may hold a
// truncated stream (limited byte count), so a read error is ignored and the
// bytes inflated so far are returned best-effort.
func decodeBody(b []byte, encoding string) ([]byte, bool) {
	if strings.ToLower(strings.TrimSpace(encoding)) != "gzip" {
		return nil, false
	}
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, false
	}
	out, _ := io.ReadAll(zr)
	return out, true
}

func (w *Wrapper) GetStatusCode() int {
	if w.statusCode == 0 {
		w.statusCode = 200
	}
	return w.statusCode
}

func (w *Wrapper) StatusCodeIsHttpOk() bool {
	statusCode := w.GetStatusCode()
	return statusCode >= 200 && statusCode < 300
}

// MarkExpected flags the response as a non-2xx that the gateway itself
// produced as expected behavior (e.g. an auth 401), letting the logging layer
// decide separately whether such responses should be logged.
func (w *Wrapper) MarkExpected() {
	w.expected = true
}

func (w *Wrapper) IsExpected() bool {
	return w.expected
}

func (w *Wrapper) GetStatusCodeErr() error {
	if w.StatusCodeIsHttpOk() {
		return nil
	}
	return statusCodeErr
}

func (w *Wrapper) GetStatusCodeStr() string {
	return strconv.Itoa(w.GetStatusCode())
}
