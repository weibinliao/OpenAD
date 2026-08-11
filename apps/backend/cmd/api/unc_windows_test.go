//go:build windows

package main

import "testing"

func TestShouldSkipUNCShareKeepsBusinessHiddenSharesOptIn(t *testing.T) {
	if !shouldSkipUNCShare("Finance$", 0, false) {
		t.Fatal("hidden business share should be skipped by default")
	}
	if shouldSkipUNCShare("Finance$", 0, true) {
		t.Fatal("hidden business share should be available when explicitly included")
	}
	for _, name := range []string{"C$", "D$", "ADMIN$", "IPC$", "PRINT$"} {
		if !shouldSkipUNCShare(name, 0, true) {
			t.Fatalf("system share %q should remain filtered", name)
		}
	}
}
