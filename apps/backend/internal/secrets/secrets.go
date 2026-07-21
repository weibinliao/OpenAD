// Package secrets provides at-rest encryption for stored credentials using
// AES-256-GCM with a locally generated key file. The key file lives in the
// application data directory with 0600 permissions; wrapping it with Windows
// DPAPI is a planned hardening step and is documented in SECURITY.md.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const keyFileName = "secret.key"

var (
	mu        sync.Mutex
	cachedKey []byte
)

// LoadOrCreateKey returns the 32-byte encryption key stored in dataDir,
// generating and persisting a new one on first use.
func LoadOrCreateKey(dataDir string) ([]byte, error) {
	mu.Lock()
	defer mu.Unlock()

	if cachedKey != nil {
		return cachedKey, nil
	}

	if strings.TrimSpace(dataDir) == "" {
		return nil, errors.New("secrets: data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}

	keyPath := filepath.Join(dataDir, keyFileName)
	if raw, err := os.ReadFile(keyPath); err == nil {
		key, decodeErr := hex.DecodeString(strings.TrimSpace(string(raw)))
		if decodeErr != nil || len(key) != 32 {
			return nil, errors.New("secrets: key file is corrupt; remove " + keyPath + " to regenerate (stored credentials will need re-entry)")
		}
		cachedKey = key
		return key, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(key)), 0o600); err != nil {
		return nil, err
	}
	cachedKey = key
	return key, nil
}

// Encrypt seals plaintext with AES-256-GCM and returns base64(nonce || ciphertext).
func Encrypt(key []byte, plaintext string) (string, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt.
func Decrypt(key []byte, encoded string) (string, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", err
	}
	if len(raw) < aead.NonceSize() {
		return "", errors.New("secrets: ciphertext too short")
	}
	nonce, ciphertext := raw[:aead.NonceSize()], raw[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("secrets: key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// ResetForTest clears the cached key so tests can use isolated directories.
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	cachedKey = nil
}
