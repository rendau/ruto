package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// NewSafePath refuses a path with a "." or ".." segment, written as is or
// percent-encoded, with 400. The router matches the path as sent and the
// backend gets it unchanged, so "/app/public/..%2fadmin" would pass the auth of
// a public "public/*" endpoint and reach "/admin" on a backend that resolves
// dot segments itself (nginx, Tomcat, ASP.NET). An encoded slash alone stays
// allowed: it is a legal part of a path parameter.
func NewSafePath() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if hasDotSegment(r.URL.EscapedPath()) {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hasDotSegment(rawPath string) bool {
	for segment := range strings.SplitSeq(rawPath, "/") {
		decoded, err := url.PathUnescape(segment)
		if err != nil {
			return true
		}
		// A decoded segment may hold "/" or "\" (a separator for IIS and
		// ASP.NET); Tomcat drops ";params", so "..;" is ".." to it.
		for part := range strings.FieldsFuncSeq(decoded, isPathSeparator) {
			part, _, _ = strings.Cut(part, ";")
			if part == "." || part == ".." {
				return true
			}
		}
	}
	return false
}

func isPathSeparator(r rune) bool {
	return r == '/' || r == '\\'
}
