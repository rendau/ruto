package core

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rendau/ruto/internal/infra/acceptenc"
)

var adminDistCandidates = []string{
	"./admin-dist",
	"./apps/admin/dist",
}

// adminCacheControl applies to every admin file. private keeps Cloudflare from
// caching it: a miss of its cache costs ~0.7s even for a 0.5KB file, going
// past it ~0.15s. no-cache lets the browser keep a copy but revalidate it each
// time (Last-Modified → 304), so a new admin build shows up right away.
const adminCacheControl = "private, no-cache"

func NewAdminSPAHandler() http.Handler {
	distPath := resolveFirstExistingDir(adminDistCandidates...)
	if distPath == "" {
		return http.NotFoundHandler()
	}

	return newAdminSPAHandler(distPath)
}

func newAdminSPAHandler(distPath string) http.Handler {
	indexPath := filepath.Join(distPath, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", adminCacheControl)

		requestPath := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")

		filePath := indexPath
		if requestPath != "." {
			fullPath := filepath.Join(distPath, requestPath)
			if stat, err := os.Stat(fullPath); err == nil && !stat.IsDir() {
				filePath = fullPath
			}
		}

		serveAdminFile(w, r, filePath)
	})
}

type precompressedEncoding struct {
	name string
	ext  string
}

// precompressedEncodings are the copies apps/admin/scripts/compress.mjs writes
// next to the build files, the preferred one first.
var precompressedEncodings = []precompressedEncoding{
	{name: "br", ext: ".br"},
	{name: "gzip", ext: ".gz"},
}

func serveAdminFile(w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Add("Vary", "Accept-Encoding")

	acceptEncoding := r.Header.Get("Accept-Encoding")
	for _, enc := range precompressedEncodings {
		if acceptenc.Accepts(acceptEncoding, enc.name) && serveEncodedFile(w, r, path, enc) {
			return
		}
	}

	http.ServeFile(w, r, path)
}

// serveEncodedFile serves the enc copy of path, if there is one. Type and
// Last-Modified come from the original, so a cached copy revalidates the same
// way whichever encoding the browser got.
func serveEncodedFile(w http.ResponseWriter, r *http.Request, path string, enc precompressedEncoding) bool {
	f, err := os.Open(path + enc.ext)
	if err != nil {
		return false
	}
	defer f.Close()

	stat, err := os.Stat(path)
	if err != nil {
		return false
	}

	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Encoding", enc.name)
	http.ServeContent(w, r, path, stat.ModTime(), f)

	return true
}

func resolveFirstExistingDir(paths ...string) string {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			return path
		}
	}

	return ""
}
