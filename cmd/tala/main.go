package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"tala/internal/app"
	"tala/internal/httpapi"
	"tala/internal/store"
)

//go:embed static/*
var staticFS embed.FS

func main() {
	addr := flag.String("addr", env("TALA_ADDR", "127.0.0.1:8080"), "listen address")
	dbPath := flag.String("db", env("TALA_DB", ".tala/tala.db"), "SQLite database path")
	flag.Parse()

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	var static http.Handler
	if sub, err := fs.Sub(staticFS, "static"); err == nil {
		static = spaFileServer(http.FS(sub))
	}

	handler := httpapi.New(app.NewServiceWithUploadDir(st, app.UploadDirForDBPath(*dbPath)), static).Routes()
	log.Printf("Tala listening on http://%s", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal(err)
	}
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func spaFileServer(root http.FileSystem) http.Handler {
	files := http.FileServer(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		file, err := root.Open(path)
		if err == nil {
			_ = file.Close()
			setStaticCacheHeaders(w, path)
			files.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(path, "assets/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		r.URL.Path = "/"
		files.ServeHTTP(w, r)
	})
}

func setStaticCacheHeaders(w http.ResponseWriter, path string) {
	if path == "index.html" {
		w.Header().Set("Cache-Control", "no-store")
		return
	}
	if strings.HasPrefix(path, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
}
