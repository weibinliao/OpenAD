package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeHostDefaultsToLoopback(t *testing.T) {
	if actual := normalizeHost(""); actual != "127.0.0.1" {
		t.Fatalf("normalizeHost(\"\") = %q, want 127.0.0.1", actual)
	}
}

func TestNormalizeHostPreservesExplicitAllInterfaceHosts(t *testing.T) {
	for _, host := range []string{"0.0.0.0", "*", "+"} {
		t.Run(host, func(t *testing.T) {
			if actual := normalizeHost(host); actual != "0.0.0.0" {
				t.Fatalf("normalizeHost(%q) = %q, want 0.0.0.0", host, actual)
			}
		})
	}
}

func TestStaticHandlerPreventsStaleDesktopAssetCaching(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<html>OpenAD</html>"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	staticHandler(root).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if actual := recorder.Header().Get("Cache-Control"); actual != "no-store, no-cache, must-revalidate" {
		t.Fatalf("Cache-Control = %q", actual)
	}
}
