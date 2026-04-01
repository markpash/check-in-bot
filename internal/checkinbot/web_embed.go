package checkinbot

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:frontend_dist
var frontendFS embed.FS

func spaHandler() http.Handler {
	sub, err := fs.Sub(frontendFS, "frontend_dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" || path == "" {
			w.Header().Set("Cache-Control", "no-store")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		if path != "/" && !strings.HasSuffix(path, "/") {
			if f, err := sub.Open(strings.TrimPrefix(path, "/")); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
