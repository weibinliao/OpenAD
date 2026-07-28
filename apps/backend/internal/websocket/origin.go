package websocket

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	allowedOriginsEnv       = "WEBSOCKET_ALLOWED_ORIGINS"
	legacyAllowedOriginsEnv = "FSA_WEBSOCKET_ALLOWED_ORIGINS"
)

type originIdentity struct {
	scheme string
	host   string
	port   string
}

type OriginPolicy struct {
	additionalOrigins map[originIdentity]struct{}
}

var defaultOriginPolicy = NewOriginPolicyFromEnv()

func NewOriginPolicy(additionalOrigins []string) (OriginPolicy, []string) {
	policy := OriginPolicy{additionalOrigins: make(map[originIdentity]struct{}, len(additionalOrigins))}
	invalidOrigins := make([]string, 0)
	for _, value := range additionalOrigins {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		origin, ok := parseOrigin(trimmed)
		if !ok {
			invalidOrigins = append(invalidOrigins, trimmed)
			continue
		}
		policy.additionalOrigins[origin] = struct{}{}
	}

	return policy, invalidOrigins
}

func NewOriginPolicyFromEnv() OriginPolicy {
	rawOrigins := strings.TrimSpace(os.Getenv(allowedOriginsEnv))
	if rawOrigins == "" {
		rawOrigins = strings.TrimSpace(os.Getenv(legacyAllowedOriginsEnv))
	}

	policy, invalidOrigins := NewOriginPolicy(splitOrigins(rawOrigins))
	for _, origin := range invalidOrigins {
		log.Printf("ignoring invalid websocket allowed origin %q", origin)
	}
	return policy
}

func CheckOrigin(request *http.Request) bool {
	return defaultOriginPolicy.CheckOrigin(request)
}

func (policy OriginPolicy) CheckOrigin(request *http.Request) bool {
	if policy.Allows(request) {
		return true
	}

	origin := ""
	if request != nil {
		origin = strings.Join(request.Header.Values("Origin"), ", ")
	}
	log.Printf("websocket origin rejected: origin=%q", origin)
	return false
}

func (policy OriginPolicy) Allows(request *http.Request) bool {
	if request == nil {
		return false
	}

	originHeaders := request.Header.Values("Origin")
	// Browsers include Origin in WebSocket handshakes. Its absence is treated as a
	// non-browser client such as a CLI or test client, which remains supported.
	if len(originHeaders) == 0 {
		return true
	}
	if len(originHeaders) != 1 {
		return false
	}

	origin, ok := parseOrigin(originHeaders[0])
	if !ok {
		return false
	}
	if isLoopbackOrigin(origin) {
		return true
	}

	requestOrigin, ok := requestOrigin(request)
	if ok && origin == requestOrigin {
		return true
	}

	_, allowed := policy.additionalOrigins[origin]
	return allowed
}

func parseOrigin(value string) (originIdentity, bool) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed == nil {
		return originIdentity{}, false
	}

	scheme := strings.ToLower(parsed.Scheme)
	if (scheme != "http" && scheme != "https") || parsed.Host == "" || parsed.Hostname() == "" {
		return originIdentity{}, false
	}
	if parsed.User != nil || parsed.Opaque != "" || parsed.Path != "" || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return originIdentity{}, false
	}
	if strings.HasSuffix(parsed.Host, ":") {
		return originIdentity{}, false
	}

	port, ok := normalizedPort(scheme, parsed.Port())
	if !ok {
		return originIdentity{}, false
	}

	return originIdentity{
		scheme: scheme,
		host:   strings.ToLower(strings.TrimSuffix(parsed.Hostname(), ".")),
		port:   port,
	}, true
}

func requestOrigin(request *http.Request) (originIdentity, bool) {
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	return parseOrigin(scheme + "://" + request.Host)
}

func normalizedPort(scheme string, port string) (string, bool) {
	if port == "" {
		if scheme == "https" {
			return "443", true
		}
		return "80", true
	}

	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return "", false
	}
	return strconv.Itoa(value), true
}

func isLoopbackOrigin(origin originIdentity) bool {
	if strings.EqualFold(origin.host, "localhost") {
		return true
	}

	ip := net.ParseIP(origin.host)
	return ip != nil && ip.IsLoopback()
}

func splitOrigins(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
}
