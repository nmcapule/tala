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
	}))

	for _, tt := range []struct {
		name     string
		path     string
		contains string
	}{
		{name: "root", path: "/", contains: "Tala app shell"},
		{name: "client route", path: "/issues/issue_123", contains: "Tala app shell"},
		{name: "asset", path: "/assets/index.js", contains: "console.log('asset');"},
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
	}
}
