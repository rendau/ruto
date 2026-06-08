package rw_wrapper

import (
	"errors"
	"net/http"
	"strconv"
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
	return w.bodyCap.String()
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
