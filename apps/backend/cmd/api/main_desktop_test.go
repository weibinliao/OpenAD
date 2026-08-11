//go:build openad_desktop

package main

import (
	"testing"
)

func TestDesktopBuildAlwaysBindsAPIToLoopback(t *testing.T) {
	t.Setenv("API_HOST", "0.0.0.0")
	t.Setenv("BIND_HOST", "*")
	t.Setenv("API_PORT", "18080")
	t.Setenv("PORT", "19090")

	if actual := resolveServerAddress(); actual != "127.0.0.1:18080" {
		t.Fatalf("desktop resolveServerAddress() = %q, want 127.0.0.1:18080", actual)
	}
}
