package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"tala/internal/app"
	"tala/internal/mcp"
	"tala/internal/store"
)

func main() {
	dbPath := flag.String("db", env("TALA_DB", ".tala/tala.db"), "SQLite database path")
	flag.Parse()

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	server := mcp.New(app.NewService(st))
	if err := server.ServeStdio(context.Background(), os.Stdin, os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
