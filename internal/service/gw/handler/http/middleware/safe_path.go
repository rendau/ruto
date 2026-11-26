package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// NewSafePath refuses ambiguous or traversal-prone paths with 400 before
// routing: dot segments ("." / ".."), empty segments ("//"), backslashes,
// semicolons (Tomcat/Spring path parameters), control characters and invalid
// percent-encodings, both raw and after percent-decoding. Because the backend
// receives the path unchanged, any of these could let a request match a public
// wildcard or "{param}" route while the backend resolves it to a protected
// endpoint. An encoded slash alone ("a%2Fb") stays allowed when it does not
// form an empty or dot segment.
func NewSafePath() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isUnsafePath(r.URL) {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isUnsafePath(u *url.URL) bool {
	if u == nil {
		return false
	}

	rawPath := u.RawPath
	if rawPath == "" {
		rawPath = u.EscapedPath()
	}

	decoded, err := url.PathUnescape(rawPath)
	if err != nil {
		return true
	}
	if u.RawPath != "" && decoded != u.Path {
		return true
	}

	return hasUnsafeChars(rawPath) || hasUnsafeChars(decoded) ||
		hasBadSegment(rawPath) || hasBadSegment(decoded)
}

func hasUnsafeChars(p string) bool {
	for i := range len(p) {
		b := p[i]
		// Control characters, backslashes (separator on IIS/ASP.NET) and
		// semicolons (path params stripped by Tomcat/Spring).
		if b < 0x20 || b == 0x7f || b == '\\' || b == ';' {
			return true
		}
	}
	return false
}

func hasBadSegment(p string) bool {
	if strings.Contains(p, "//") {
		return true
	}
	for segment := range strings.SplitSeq(p, "/") {
		if segment == "." || segment == ".." {
			return true
		}
	}
	return false
}
