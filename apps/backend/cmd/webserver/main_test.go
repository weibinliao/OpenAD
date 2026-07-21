package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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
