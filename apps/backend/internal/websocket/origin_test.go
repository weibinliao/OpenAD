package websocket

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOriginPolicyAllowsExpectedOrigins(t *testing.T) {
	policy, invalidOrigins := NewOriginPolicy(nil)
	require.Empty(t, invalidOrigins)

	tests := []struct {
		name          string
		requestURL    string
		origin        string
		includeOrigin bool
		allowed       bool
	}{
		{name: "same origin", requestURL: "http://api.example.com:18080/api/scan/ws", origin: "http://api.example.com:18080", includeOrigin: true, allowed: true},
		{name: "same origin ignores case", requestURL: "http://api.example.com:18080/api/scan/ws", origin: "HTTP://API.EXAMPLE.COM:18080", includeOrigin: true, allowed: true},
		{name: "same host with different port", requestURL: "http://api.example.com:18080/api/scan/ws", origin: "http://api.example.com:18081", includeOrigin: true, allowed: false},
		{name: "same host with different scheme", requestURL: "https://api.example.com/api/scan/ws", origin: "http://api.example.com", includeOrigin: true, allowed: false},
		{name: "localhost with different port", requestURL: "http://192.0.2.20:18080/api/scan/ws", origin: "http://localhost:3010", includeOrigin: true, allowed: true},
		{name: "ipv4 loopback with desktop port", requestURL: "http://127.0.0.1:18080/api/scan/ws", origin: "http://127.0.0.1:43110", includeOrigin: true, allowed: true},
		{name: "ipv6 loopback", requestURL: "http://127.0.0.1:18080/api/scan/ws", origin: "http://[::1]:43110", includeOrigin: true, allowed: true},
		{name: "external origin", requestURL: "http://192.0.2.20:18080/api/scan/ws", origin: "https://evil.example", includeOrigin: true, allowed: false},
		{name: "missing origin", requestURL: "http://192.0.2.20:18080/api/scan/ws", includeOrigin: false, allowed: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.requestURL, nil)
			if test.includeOrigin {
				request.Header.Set("Origin", test.origin)
			}

			assert.Equal(t, test.allowed, policy.Allows(request))
		})
	}
}

func TestOriginPolicyAllowsConfiguredAdditionalOrigin(t *testing.T) {
	policy, invalidOrigins := NewOriginPolicy([]string{"https://Console.Example.com:8443"})
	require.Empty(t, invalidOrigins)

	request := httptest.NewRequest(http.MethodGet, "http://192.0.2.20:18080/api/scan/ws", nil)
	request.Header.Set("Origin", "https://console.example.com:8443")

	assert.True(t, policy.Allows(request))

	request.Header.Set("Origin", "https://console.example.com:9443")
	assert.False(t, policy.Allows(request))
}

func TestOriginPolicyFromEnvUsesPrimaryAndLegacyNames(t *testing.T) {
	t.Run("primary", func(t *testing.T) {
		t.Setenv("WEBSOCKET_ALLOWED_ORIGINS", "https://primary.example:8443")
		t.Setenv("FSA_WEBSOCKET_ALLOWED_ORIGINS", "https://legacy.example:8443")

		policy := NewOriginPolicyFromEnv()
		assert.True(t, policy.Allows(requestWithOrigin("https://primary.example:8443")))
		assert.False(t, policy.Allows(requestWithOrigin("https://legacy.example:8443")))
	})

	t.Run("legacy fallback", func(t *testing.T) {
		t.Setenv("WEBSOCKET_ALLOWED_ORIGINS", "")
		t.Setenv("FSA_WEBSOCKET_ALLOWED_ORIGINS", "https://legacy.example:8443")

		policy := NewOriginPolicyFromEnv()
		assert.True(t, policy.Allows(requestWithOrigin("https://legacy.example:8443")))
	})
}

func TestOriginPolicyRejectsMalformedOrigins(t *testing.T) {
	policy, invalidOrigins := NewOriginPolicy([]string{"https://trusted.example/path"})
	require.Equal(t, []string{"https://trusted.example/path"}, invalidOrigins)

	for _, origin := range []string{
		"",
		"null",
		"://missing-scheme.example",
		"ftp://files.example",
		"https://",
		"https://user:password@example.com",
		"https://example.com/path",
		"https://example.com?query=value",
		"https://example.com#fragment",
		"https://example.com:invalid",
	} {
		t.Run(origin, func(t *testing.T) {
			request := requestWithOrigin(origin)
			assert.False(t, policy.Allows(request))
		})
	}
}

func TestOriginPolicyLogsOnlyRejectedOrigin(t *testing.T) {
	policy, invalidOrigins := NewOriginPolicy(nil)
	require.Empty(t, invalidOrigins)

	request := httptest.NewRequest(http.MethodGet, "http://192.0.2.20:18080/api/scan/ws?scan_id=sensitive-session", nil)
	request.Header.Set("Origin", "https://evil.example")

	var output bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	originalPrefix := log.Prefix()
	log.SetOutput(&output)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
		log.SetPrefix(originalPrefix)
	})

	assert.False(t, policy.CheckOrigin(request))
	assert.Contains(t, output.String(), `origin="https://evil.example"`)
	assert.False(t, strings.Contains(output.String(), "sensitive-session"))
	assert.False(t, strings.Contains(output.String(), "192.0.2.20"))
}

func requestWithOrigin(origin string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "http://192.0.2.20:18080/api/scan/ws", nil)
	request.Header.Set("Origin", origin)
	return request
}
