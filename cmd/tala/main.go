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
	dbPath := flag.String("db", env("TALA_DB", "tala.db"), "SQLite database path")
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

	handler := httpapi.New(app.NewService(st), static).Routes()
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
			files.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		files.ServeHTTP(w, r)
	})
}
