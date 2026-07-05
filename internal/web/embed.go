// Package web embeds the built React SPA and serves it with client-side-route
// fallback to index.html. The API and system routes are mounted ahead of this
// handler, so it only ever handles UI paths.
package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// spaHandler serves embedded static assets, falling back to index.html for any
// path that does not resolve to a real file (SPA client-side routing).
type spaHandler struct {
	fsys       fs.FS
	fileServer http.Handler
	index      []byte
}

// Handler returns an http.Handler serving the embedded SPA.
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, fmt.Errorf("locating embedded dist: %w", err)
	}
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return nil, fmt.Errorf("embedded SPA missing index.html: %w", err)
	}
	return &spaHandler{
		fsys:       sub,
		fileServer: http.FileServer(http.FS(sub)),
		index:      index,
	}, nil
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if upath == "" {
		h.writeIndex(w)
		return
	}
	if f, err := h.fsys.Open(upath); err == nil {
		info, statErr := f.Stat()
		_ = f.Close()
		if statErr == nil && !info.IsDir() {
			h.fileServer.ServeHTTP(w, r)
			return
		}
	}
	// Unknown path → hand it to the SPA router.
	h.writeIndex(w)
}

func (h *spaHandler) writeIndex(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.index)
}
