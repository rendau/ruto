package middleware

import (
	"net/http"
	"slices"
)

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	result := h
	for _, mw := range slices.Backward(middlewares) {
		result = mw(result)
	}
	return result
}
