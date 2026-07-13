package web

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler serves the embedded SPA build from internal/web/dist.
// Unknown paths fall back to index.html so client-side routing works,
// except for paths that look like assets (contain a dot) which return 404.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("web: dist subdir not found: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	indexBytes := readIndex(sub)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := strings.TrimPrefix(r.URL.Path, "/")
		if up == "" {
			up = "index.html"
		}

		if _, statErr := fs.Stat(sub, up); statErr != nil {
			if isAsset(up) {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(indexBytes)
			return
		}

		if up == "index.html" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func isAsset(p string) bool {
	return strings.Contains(path.Base(p), ".")
}

func readIndex(fsys fs.FS) []byte {
	b, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return []byte("<!doctype html><title>AstreoGateway</title><h1>UI not built</h1>")
	}
	return bytes.TrimSpace(b)
}