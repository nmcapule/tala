package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"tala/internal/mcp"
)

func main() {
	dbPath := flag.String("db", env("TALA_DB", ".tala/tala.db"), "SQLite database path")
	flag.Parse()

	server, err := mcp.NewLazy(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()

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
