package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	dir := t.TempDir()
	key, err := LoadOrCreateKey(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateKey: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}

	encrypted, err := Encrypt(key, "P@ssw0rd!中文")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if encrypted == "P@ssw0rd!中文" {
		t.Fatal("ciphertext equals plaintext")
	}

	plain, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if plain != "P@ssw0rd!中文" {
		t.Fatalf("round trip mismatch: %q", plain)
	}
}

func TestKeyPersistsAcrossLoads(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	dir := t.TempDir()
	first, err := LoadOrCreateKey(dir)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}

	ResetForTest()
	second, err := LoadOrCreateKey(dir)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("key changed between loads")
	}

	if _, err := os.Stat(filepath.Join(dir, "secret.key")); err != nil {
		t.Fatalf("key file missing: %v", err)
	}
}

func TestDecryptRejectsGarbage(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	key, err := LoadOrCreateKey(t.TempDir())
	if err != nil {
		t.Fatalf("LoadOrCreateKey: %v", err)
	}
	if _, err := Decrypt(key, "bm90LXJlYWwtY2lwaGVydGV4dA=="); err == nil {
		t.Fatal("expected error decrypting garbage")
	}
}
