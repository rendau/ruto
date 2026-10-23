package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminSPAHandler_CacheControl(t *testing.T) {
	dist := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dist, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dist, "index.html"), []byte("<html>index</html>"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dist, "assets", "index-abc.js"), []byte("console.log(1)"), 0o644))

	handler := newAdminSPAHandler(dist)

	tests := []struct {
		path     string
		wantBody string
	}{
		{path: "/", wantBody: "<html>index</html>"},
		{path: "/root", wantBody: "<html>index</html>"},
		{path: "/apps/123", wantBody: "<html>index</html>"},
		{path: "/assets/index-abc.js", wantBody: "console.log(1)"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.wantBody, rec.Body.String())
			assert.Equal(t, "private, no-cache", rec.Header().Get("Cache-Control"))

			lastModified := rec.Header().Get("Last-Modified")
			require.NotEmpty(t, lastModified)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("If-Modified-Since", lastModified)
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusNotModified, rec.Code)
			assert.Equal(t, "private, no-cache", rec.Header().Get("Cache-Control"))
		})
	}
}
