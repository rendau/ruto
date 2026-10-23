package core

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	fileServer := http.FileServer(http.Dir(distPath))
	indexPath := filepath.Join(distPath, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", adminCacheControl)

		requestPath := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")

		if requestPath == "." {
			http.ServeFile(w, r, indexPath)
			return
		}

		fullPath := filepath.Join(distPath, requestPath)
		if stat, err := os.Stat(fullPath); err == nil && !stat.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, indexPath)
	})
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
