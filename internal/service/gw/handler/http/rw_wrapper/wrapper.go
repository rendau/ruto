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
}

func New(w http.ResponseWriter) *Wrapper {
	return &Wrapper{ResponseWriter: w}
}

func (w *Wrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
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

func (w *Wrapper) GetStatusCodeErr() error {
	if w.StatusCodeIsHttpOk() {
		return nil
	}
	return statusCodeErr
}

func (w *Wrapper) GetStatusCodeStr() string {
	return strconv.Itoa(w.GetStatusCode())
}
