// Package web serves the compiled frontend; the router needs only the Go binary.
package web

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed dist/*
var files embed.FS

func Handler() http.Handler {
	root, _ := fs.Sub(files, "dist")
	server := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Méthode interdite", 405)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "." || name == "" {
			name = "index.html"
		}
		if strings.HasPrefix(name, "assets/") {
			if _, err := fs.Stat(root, name); err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			w.Header().Set("Vary", "Accept-Encoding")
			if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				if packed, err := fs.ReadFile(root, name+".gz"); err == nil {
					w.Header().Set("Content-Encoding", "gzip")
					if strings.HasSuffix(name, ".js") {
						w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
					}
					if strings.HasSuffix(name, ".css") {
						w.Header().Set("Content-Type", "text/css; charset=utf-8")
					}
					http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(packed))
					return
				}
			}
			server.ServeHTTP(w, r)
			return
		}
		switch name {
		case "index.html", "chauffage", "installation", "analyse", "pellets", "meteo", "system":
		default:
			http.NotFound(w, r)
			return
		}
		data, err := fs.ReadFile(root, "index.html")
		if err != nil {
			http.Error(w, "Frontend non compilé", 503)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
	})
}
