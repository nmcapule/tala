package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestEnvUsesTrimmedValueOrFallback(t *testing.T) {
	t.Setenv("TALA_TEST_ENV", "  configured  ")
	if got := env("TALA_TEST_ENV", "fallback"); got != "configured" {
		t.Fatalf("env returned %q, want configured", got)
	}

	t.Setenv("TALA_TEST_ENV", "   ")
	if got := env("TALA_TEST_ENV", "fallback"); got != "fallback" {
		t.Fatalf("blank env returned %q, want fallback", got)
	}
}

func TestSPAFileServerServesAssetsAndFallsBackToIndex(t *testing.T) {
	handler := spaFileServer(http.FS(fstest.MapFS{
		"index.html": {
			Data: []byte("<main>Tala app shell</main>"),
		},
		"assets/index.js": {
			Data: []byte("console.log('asset');"),
		},
		"assets/logo.1a2b3c4d.png": {
			Data: []byte("png"),
		},
	}))

	for _, tt := range []struct {
		name         string
		path         string
		contains     string
		cacheControl string
	}{
		{name: "root", path: "/", contains: "Tala app shell", cacheControl: "no-store"},
		{name: "client route", path: "/issues/issue_123", contains: "Tala app shell", cacheControl: "no-store"},
		{name: "stable asset", path: "/assets/index.js", contains: "console.log('asset');", cacheControl: "no-cache"},
		{name: "versioned asset", path: "/assets/logo.1a2b3c4d.png", contains: "png", cacheControl: "public, max-age=31536000, immutable"},
	} {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("%s: got status %d, want 200", tt.name, res.Code)
		}
		if !strings.Contains(res.Body.String(), tt.contains) {
			t.Fatalf("%s: body %q does not contain %q", tt.name, res.Body.String(), tt.contains)
		}
		if got := res.Header().Get("Cache-Control"); got != tt.cacheControl {
			t.Fatalf("%s: Cache-Control = %q, want %q", tt.name, got, tt.cacheControl)
		}
	}
}
