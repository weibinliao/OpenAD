package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

var requestSequence uint64
var endpointLimiter = newWindowRateLimiter(120, time.Minute)

func securityHeadersMiddleware() gin.HandlerFunc {
	return func(context *gin.Context) {
		context.Header("X-Content-Type-Options", "nosniff")
		context.Header("X-Frame-Options", "DENY")
		context.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		context.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		context.Header("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		context.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		if context.Request.TLS != nil {
			context.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		context.Next()
	}
}

type networkAdmissionPolicy struct {
	Enabled       bool
	Raw           string
	DenyRaw       string
	AllowRules    []string
	DenyRules     []string
	AllowAll      bool
	AllowLoopback bool
	AllowPrivate  bool
	DenyAll       bool
	DenyPrivate   bool
	AllowIPs      []net.IP
	AllowNetworks []*net.IPNet
	DenyIPs       []net.IP
	DenyNetworks  []*net.IPNet
}

type networkAdmissionController struct {
	mutex  sync.RWMutex
	policy networkAdmissionPolicy
}

type networkAdmissionConfig struct {
	Enabled    bool     `json:"enabled"`
	AllowRules []string `json:"allow_rules"`
	DenyRules  []string `json:"deny_rules"`
}

type networkAdmissionUpdateRequest struct {
	Enabled    bool     `json:"enabled"`
	AllowRules []string `json:"allow_rules"`
	DenyRules  []string `json:"deny_rules"`
}

type compiledNetworkRules struct {
	Rules         []string
	AllowAll      bool
	AllowLoopback bool
	AllowPrivate  bool
	IPs           []net.IP
	Networks      []*net.IPNet
	Errors        []string
}

func newNetworkAdmissionController(policy networkAdmissionPolicy) *networkAdmissionController {
	return &networkAdmissionController{policy: policy}
}

func (controller *networkAdmissionController) Policy() networkAdmissionPolicy {
	controller.mutex.RLock()
	defer controller.mutex.RUnlock()

	return controller.policy
}

func (controller *networkAdmissionController) Allows(ip net.IP) bool {
	return controller.Policy().Allows(ip)
}

func (controller *networkAdmissionController) Update(policy networkAdmissionPolicy) {
	controller.mutex.Lock()
	defer controller.mutex.Unlock()

	controller.policy = policy
}

func networkAdmissionPolicyFromEnv() networkAdmissionPolicy {
	rawValue := strings.TrimSpace(os.Getenv("NETWORK_ACL"))
	if rawValue == "" {
		rawValue = strings.TrimSpace(os.Getenv("FSA_NETWORK_ACL"))
	}
	denyValue := strings.TrimSpace(os.Getenv("NETWORK_ACL_DENY"))
	if denyValue == "" {
		denyValue = strings.TrimSpace(os.Getenv("FSA_NETWORK_ACL_DENY"))
	}
	if rawValue == "" {
		return networkAdmissionPolicy{}
	}

	policy, errors := buildNetworkAdmissionPolicy(true, splitNetworkRules(rawValue), splitNetworkRules(denyValue))
	if len(errors) > 0 {
		log.Printf("network admission env contained invalid rules: %s", strings.Join(errors, "; "))
	}
	return policy
}

func loadNetworkAdmissionPolicy() networkAdmissionPolicy {
	configPath := networkAdmissionConfigPath()
	if configPath == "" {
		return networkAdmissionPolicyFromEnv()
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("failed to read network admission config %s: %v", configPath, err)
		}
		return networkAdmissionPolicyFromEnv()
	}

	var config networkAdmissionConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("failed to parse network admission config %s: %v", configPath, err)
		return networkAdmissionPolicyFromEnv()
	}

	policy, validationErrors := buildNetworkAdmissionPolicy(config.Enabled, config.AllowRules, config.DenyRules)
	if len(validationErrors) > 0 {
		log.Printf("network admission config contained invalid rules: %s", strings.Join(validationErrors, "; "))
	}
	return policy
}

func networkAdmissionConfigPath() string {
	if value := strings.TrimSpace(os.Getenv("NETWORK_ACL_CONFIG")); value != "" {
		return value
	}
	return filepath.Clean(filepath.Join("..", "..", ".local", "network-admission.json"))
}

func saveNetworkAdmissionPolicy(policy networkAdmissionPolicy) error {
	configPath := networkAdmissionConfigPath()
	if configPath == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}

	config := networkAdmissionConfig{
		Enabled:    policy.Enabled,
		AllowRules: policy.AllowRules,
		DenyRules:  policy.DenyRules,
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0o600)
}

func buildNetworkAdmissionPolicy(enabled bool, allowRules []string, denyRules []string) (networkAdmissionPolicy, []string) {
	allow := compileNetworkRules(allowRules)
	deny := compileNetworkRules(denyRules)
	policy := networkAdmissionPolicy{
		Enabled:       enabled,
		Raw:           strings.Join(allow.Rules, ","),
		DenyRaw:       strings.Join(deny.Rules, ","),
		AllowRules:    allow.Rules,
		DenyRules:     deny.Rules,
		AllowAll:      allow.AllowAll,
		AllowLoopback: true,
		AllowPrivate:  allow.AllowPrivate,
		DenyAll:       deny.AllowAll,
		DenyPrivate:   deny.AllowPrivate,
		AllowIPs:      allow.IPs,
		AllowNetworks: allow.Networks,
		DenyIPs:       deny.IPs,
		DenyNetworks:  deny.Networks,
	}
	return policy, append(allow.Errors, deny.Errors...)
}

func splitNetworkRules(value string) []string {
	rawItems := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ';'
	})
	items := make([]string, 0, len(rawItems))
	for _, raw := range rawItems {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	return items
}

func compileNetworkRules(rules []string) compiledNetworkRules {
	compiled := compiledNetworkRules{Rules: make([]string, 0, len(rules))}
	seen := make(map[string]struct{}, len(rules))
	for _, item := range rules {
		rule := strings.TrimSpace(item)
		if rule == "" {
			continue
		}
		if _, exists := seen[strings.ToLower(rule)]; exists {
			continue
		}
		seen[strings.ToLower(rule)] = struct{}{}
		compiled.Rules = append(compiled.Rules, rule)
		switch strings.ToLower(rule) {
		case "*", "all", "any":
			compiled.AllowAll = true
			continue
		case "loopback", "localhost", "local":
			compiled.AllowLoopback = true
			continue
		case "private", "lan", "rfc1918":
			compiled.AllowPrivate = true
			continue
		}

		if strings.Contains(rule, "/") {
			if _, network, err := net.ParseCIDR(rule); err == nil {
				compiled.Networks = append(compiled.Networks, network)
			} else {
				compiled.Errors = append(compiled.Errors, fmt.Sprintf("invalid CIDR %q", rule))
			}
			continue
		}

		if ip := net.ParseIP(rule); ip != nil {
			compiled.IPs = append(compiled.IPs, ip)
			continue
		}

		compiled.Errors = append(compiled.Errors, fmt.Sprintf("invalid rule %q", rule))
	}

	return compiled
}

func networkAdmissionMiddleware(controller *networkAdmissionController) gin.HandlerFunc {
	return func(context *gin.Context) {
		policy := controller.Policy()
		if !policy.Enabled {
			context.Next()
			return
		}

		clientIP := clientIPFromRemoteAddr(context.Request.RemoteAddr)
		if clientIP == nil || !policy.Allows(clientIP) {
			context.JSON(http.StatusForbidden, gin.H{
				"error":     "network admission denied",
				"client_ip": clientIPString(clientIP, context.Request.RemoteAddr),
				"policy":    policy.AllowRules,
			})
			context.Abort()
			return
		}

		context.Next()
	}
}

func (policy networkAdmissionPolicy) Allows(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	if !policy.Enabled {
		return true
	}
	if policy.matchesDeny(ip) {
		return false
	}
	if policy.AllowAll {
		return true
	}
	if policy.AllowPrivate && (ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		return true
	}
	for _, allowedIP := range policy.AllowIPs {
		if allowedIP.Equal(ip) {
			return true
		}
	}
	for _, network := range policy.AllowNetworks {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

func (policy networkAdmissionPolicy) matchesDeny(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if policy.DenyAll {
		return true
	}
	if policy.DenyPrivate && (ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		return true
	}
	for _, deniedIP := range policy.DenyIPs {
		if deniedIP.Equal(ip) {
			return true
		}
	}
	for _, network := range policy.DenyNetworks {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

func (policy networkAdmissionPolicy) denyPrivate() bool {
	for _, rule := range policy.DenyRules {
		switch strings.ToLower(strings.TrimSpace(rule)) {
		case "private", "lan", "rfc1918":
			return true
		}
	}
	return false
}

func clientIPFromRemoteAddr(remoteAddr string) net.IP {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return nil
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	host = strings.Trim(host, "[]")

	return net.ParseIP(host)
}

func clientIPString(ip net.IP, fallback string) string {
	if ip != nil {
		return ip.String()
	}
	return strings.TrimSpace(fallback)
}

func handleNetworkAdmission(controller *networkAdmissionController) gin.HandlerFunc {
	return func(context *gin.Context) {
		policy := controller.Policy()
		clientIP := clientIPFromRemoteAddr(context.Request.RemoteAddr)
		context.JSON(http.StatusOK, gin.H{
			"enabled":        policy.Enabled,
			"rules":          policy.AllowRules,
			"allow_rules":    policy.AllowRules,
			"deny_rules":     policy.DenyRules,
			"raw":            policy.Raw,
			"deny_raw":       policy.DenyRaw,
			"allow_all":      policy.AllowAll,
			"allow_loopback": policy.AllowLoopback,
			"allow_private":  policy.AllowPrivate,
			"client_ip":      clientIPString(clientIP, context.Request.RemoteAddr),
			"client_allowed": policy.Allows(clientIP),
		})
	}
}

func handleUpdateNetworkAdmission(controller *networkAdmissionController) gin.HandlerFunc {
	return func(context *gin.Context) {
		var request networkAdmissionUpdateRequest
		if err := context.ShouldBindJSON(&request); err != nil {
			context.JSON(http.StatusBadRequest, gin.H{"error": "invalid network admission payload"})
			return
		}

		policy, validationErrors := buildNetworkAdmissionPolicy(request.Enabled, request.AllowRules, request.DenyRules)
		if len(validationErrors) > 0 {
			context.JSON(http.StatusBadRequest, gin.H{"error": "invalid network admission rules", "details": validationErrors})
			return
		}

		clientIP := clientIPFromRemoteAddr(context.Request.RemoteAddr)
		if clientIP != nil && !policy.Allows(clientIP) {
			context.JSON(http.StatusBadRequest, gin.H{
				"error":     "refusing to save a policy that would block the current client",
				"client_ip": clientIP.String(),
			})
			return
		}

		if err := saveNetworkAdmissionPolicy(policy); err != nil {
			context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		controller.Update(policy)
		context.JSON(http.StatusOK, gin.H{
			"enabled":        policy.Enabled,
			"rules":          policy.AllowRules,
			"allow_rules":    policy.AllowRules,
			"deny_rules":     policy.DenyRules,
			"raw":            policy.Raw,
			"deny_raw":       policy.DenyRaw,
			"allow_all":      policy.AllowAll,
			"allow_loopback": policy.AllowLoopback,
			"allow_private":  policy.AllowPrivate,
			"client_ip":      clientIPString(clientIP, context.Request.RemoteAddr),
			"client_allowed": policy.Allows(clientIP),
		})
	}
}

func corsMiddleware() gin.HandlerFunc {
	allowedOrigins := parseAllowedOrigins(os.Getenv("ALLOW_ORIGINS"))
	allowWildcard := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"
	allowMethods := "GET, POST, PUT, OPTIONS"
	allowHeaders := "Content-Type, X-Request-ID"

	return func(context *gin.Context) {
		origin := strings.TrimSpace(context.GetHeader("Origin"))
		if allowWildcard {
			context.Header("Access-Control-Allow-Origin", "*")
		} else if origin != "" && containsFoldInsensitive(allowedOrigins, origin) {
			context.Header("Access-Control-Allow-Origin", origin)
			context.Header("Vary", "Origin")
		}
		context.Header("Access-Control-Allow-Methods", allowMethods)
		context.Header("Access-Control-Allow-Headers", allowHeaders)
		if context.Request.Method == http.MethodOptions {
			context.AbortWithStatus(http.StatusNoContent)
			return
		}

		context.Next()
	}
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(context *gin.Context) {
		requestID := strings.TrimSpace(context.GetHeader("X-Request-ID"))
		if requestID == "" {
			sequence := atomic.AddUint64(&requestSequence, 1)
			requestID = fmt.Sprintf("req-%d-%d", time.Now().UTC().UnixNano(), sequence)
		}

		context.Set("request_id", requestID)
		context.Header("X-Request-ID", requestID)
		context.Next()
	}
}

func endpointProtectionMiddleware() gin.HandlerFunc {
	protectedPostPaths := map[string]struct{}{
		"/api/scan":                          {},
		"/api/scan/:id/cancel":               {},
		"/api/ad/users/query":                {},
		"/api/ad/users/query/async":          {},
		"/api/ad/groups/query":               {},
		"/api/ad/groups/members":             {},
		"/api/ad/groups/members/export":      {},
		"/api/ad/tree":                       {},
		"/api/ad/tree/explain":               {},
		"/api/ad/jobs/:id/retry":             {},
		"/api/ad/jobs/:id/cancel":            {},
		"/api/audit/requests/export/summary": {},
		"/api/file-activity/events/query":    {},
		"/api/export/download":               {},
		"/api/export/summary":                {},
		"/api/risk-findings/upsert":          {},
		"/api/risk-findings/import":          {},
	}

	return func(context *gin.Context) {
		if context.Request.Method != http.MethodPost {
			context.Next()
			return
		}

		path := context.FullPath()
		if path == "" {
			path = context.Request.URL.Path
		}
		if _, exists := protectedPostPaths[path]; !exists {
			context.Next()
			return
		}

		clientKey := strings.TrimSpace(context.ClientIP())
		if clientKey == "" {
			clientKey = "unknown"
		}
		if !endpointLimiter.Allow(clientKey + "::" + path) {
			context.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
				"path":  path,
			})
			context.Abort()
			return
		}

		context.Next()
	}
}

func auditLogMiddleware() gin.HandlerFunc {
	return func(context *gin.Context) {
		startedAt := time.Now()
		context.Next()

		requestID := context.GetString("request_id")
		if requestID == "" {
			requestID = "-"
		}

		duration := time.Since(startedAt).Milliseconds()
		log.Printf(
			"request_id=%s method=%s path=%s status=%d duration_ms=%d client=%s user_agent=%q",
			requestID,
			context.Request.Method,
			context.Request.URL.Path,
			context.Writer.Status(),
			duration,
			context.ClientIP(),
			context.Request.UserAgent(),
		)
		auditEntries.Add(auditEntry{
			RequestID:  requestID,
			Method:     context.Request.Method,
			Path:       context.Request.URL.Path,
			Status:     context.Writer.Status(),
			DurationMS: duration,
			ClientIP:   context.ClientIP(),
			UserAgent:  context.Request.UserAgent(),
			Timestamp:  time.Now().UTC(),
		})
	}
}

func parseAllowedOrigins(value string) []string {
	localOrigins := []string{
		"http://localhost:3010",
		"http://127.0.0.1:3010",
		"http://[::1]:3010",
		"http://localhost:43110",
		"http://127.0.0.1:43110",
		"http://[::1]:43110",
	}
	if strings.TrimSpace(value) == "" {
		return localOrigins
	}

	rawItems := strings.Split(value, ",")
	items := make([]string, 0, len(rawItems))
	for _, raw := range rawItems {
		trimmed := strings.TrimSpace(raw)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	if len(items) == 0 {
		return localOrigins
	}

	return items
}

func containsFoldInsensitive(items []string, expected string) bool {
	for _, item := range items {
		if strings.EqualFold(item, expected) {
			return true
		}
	}

	return false
}

type windowRateLimiter struct {
	limit   int
	window  time.Duration
	records map[string][]time.Time
	mutex   sync.Mutex
}

func newWindowRateLimiter(limit int, window time.Duration) *windowRateLimiter {
	if limit < 1 {
		limit = 20
	}
	if window < time.Second {
		window = time.Minute
	}

	return &windowRateLimiter{
		limit:   limit,
		window:  window,
		records: make(map[string][]time.Time),
	}
}

func (limiter *windowRateLimiter) Allow(key string) bool {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		trimmedKey = "unknown"
	}

	now := time.Now().UTC()
	threshold := now.Add(-limiter.window)

	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	values := limiter.records[trimmedKey]
	recent := values[:0]
	for _, value := range values {
		if value.After(threshold) {
			recent = append(recent, value)
		}
	}

	if len(recent) >= limiter.limit {
		limiter.records[trimmedKey] = recent
		return false
	}

	recent = append(recent, now)
	limiter.records[trimmedKey] = recent
	return true
}
