package api

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:static
var embeddedFiles embed.FS

func SetupRouter(h *Handler, uiDistDir string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/system/status", h.HandleSystemStatus)
	mux.HandleFunc("/api/jobs/stream", h.HandleJobsStream)
	mux.HandleFunc("/api/jobs/submit", h.HandleSubmit)
	mux.HandleFunc("/api/jobs/", h.HandleJobs)
	mux.HandleFunc("/api/download/", h.HandleDownload)

	// Try embedded static assets first (production binary)
	subFS, err := fs.Sub(embeddedFiles, "static")
	if err == nil {
		if _, statErr := subFS.Open("index.html"); statErr == nil {
			fileServer := http.FileServer(http.FS(subFS))
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				clean := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")
				if _, openErr := subFS.Open(clean); openErr != nil {
					// SPA fallback
					indexData, _ := fs.ReadFile(subFS, "index.html")
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					_, _ = w.Write(indexData)
					return
				}
				fileServer.ServeHTTP(w, r)
			})
			return EnableCORS(mux)
		}
	}

	// Development fallback: serve from local ui/dist on disk
	if uiDistDir != "" {
		if _, statErr := os.Stat(uiDistDir); statErr == nil {
			fileServer := http.FileServer(http.Dir(uiDistDir))
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				path := filepath.Join(uiDistDir, r.URL.Path)
				if _, err := os.Stat(path); os.IsNotExist(err) {
					http.ServeFile(w, r, filepath.Join(uiDistDir, "index.html"))
					return
				}
				fileServer.ServeHTTP(w, r)
			})
		}
	}

	return EnableCORS(mux)
}
