package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	db "github.com/weibinliao/OpenAD/internal/database"
	"github.com/gin-gonic/gin"
)

func main() {
	if err := db.Init(); err != nil {
		log.Printf("database unavailable, continuing without persistence: %v", err)
	}

	router := newApplication(applicationDependencies{}).router()
	address := resolveServerAddress()

	log.Printf("Server starting on %s", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

func resolveServerAddress() string {
	host := strings.TrimSpace(os.Getenv("API_HOST"))
	if host == "" {
		host = strings.TrimSpace(os.Getenv("BIND_HOST"))
	}
	if host == "*" {
		host = "0.0.0.0"
	}

	for _, key := range []string{"API_PORT", "PORT"} {
		rawValue := strings.TrimSpace(os.Getenv(key))
		if rawValue == "" {
			continue
		}

		port, err := strconv.Atoi(rawValue)
		if err != nil || port < 1 || port > 65535 {
			log.Printf("invalid %s=%q, fallback to next/default port", key, rawValue)
			continue
		}

		if host != "" {
			return net.JoinHostPort(host, strconv.Itoa(port))
		}

		return fmt.Sprintf(":%d", port)
	}

	if host != "" {
		return net.JoinHostPort(host, "18080")
	}

	return ":18080"
}

func (application *application) handleListDirectories(context *gin.Context) {
	requestPath := strings.TrimSpace(context.Query("path"))

	if requestPath == "" {
		context.JSON(http.StatusOK, gin.H{
			"path":   "",
			"items":  listRootDirectories(),
			"parent": "",
		})
		return
	}

	info, err := os.Stat(requestPath)
	if err != nil {
		if isPermissionDeniedError(err) {
			context.JSON(http.StatusForbidden, gin.H{"error": formatDirectoryAccessDeniedMessage(requestPath, err)})
			return
		}
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !info.IsDir() {
		context.JSON(http.StatusBadRequest, gin.H{"error": "path is not a directory"})
		return
	}

	entries, err := os.ReadDir(requestPath)
	if err != nil {
		if isPermissionDeniedError(err) {
			context.JSON(http.StatusForbidden, gin.H{"error": formatDirectoryAccessDeniedMessage(requestPath, err)})
			return
		}
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]gin.H, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(requestPath, entry.Name())
		items = append(items, gin.H{
			"name": entry.Name(),
			"path": fullPath,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		left := strings.ToLower(items[i]["name"].(string))
		right := strings.ToLower(items[j]["name"].(string))
		return left < right
	})

	context.JSON(http.StatusOK, gin.H{
		"path":   requestPath,
		"items":  items,
		"parent": filepath.Dir(requestPath),
	})
}

func listRootDirectories() []gin.H {
	if runtime.GOOS == "windows" {
		roots := make([]gin.H, 0, 8)
		for drive := 'A'; drive <= 'Z'; drive++ {
			drivePath := fmt.Sprintf("%c:\\", drive)
			if _, err := os.Stat(drivePath); err == nil {
				roots = append(roots, gin.H{
					"name": drivePath,
					"path": drivePath,
				})
			}
		}

		return roots
	}

	return []gin.H{{"name": "/", "path": "/"}}
}

func handleHealth(context *gin.Context) {
	context.JSON(http.StatusOK, gin.H{
		"service":        "openad",
		"status":         "healthy",
		"database":       db.Ready(),
		"database_store": db.StoreDescription,
	})
}

func handleRuntimeIdentity(context *gin.Context) {
	hostname, _ := os.Hostname()
	currentUser, currentUserErr := user.Current()
	username := strings.TrimSpace(os.Getenv("USERNAME"))
	domain := strings.TrimSpace(os.Getenv("USERDOMAIN"))
	localIPv4Addresses := collectLocalIPv4Addresses()

	if currentUserErr == nil && currentUser != nil {
		if strings.TrimSpace(currentUser.Username) != "" {
			username = strings.TrimSpace(currentUser.Username)
		}
		if domain == "" {
			if idx := strings.Index(currentUser.Username, `\`); idx > 0 {
				domain = currentUser.Username[:idx]
			}
		}
	}

	if username == "" {
		username = "unknown"
	}

	accountName := username
	if domain != "" && !strings.Contains(username, `\`) {
		accountName = domain + `\` + username
	}

	preferredHost := hostname
	if len(localIPv4Addresses) > 0 {
		preferredHost = localIPv4Addresses[0]
	}

	context.JSON(http.StatusOK, gin.H{
		"account_name": accountName,
		"username":     username,
		"domain":       domain,
		"host":         hostname,
		"preferred_host": preferredHost,
		"local_ipv4_addresses": localIPv4Addresses,
		"goos":         runtime.GOOS,
		"note":         "UNC browsing and scanning use this backend runtime identity; AD credentials are only used for directory expansion and principal resolution",
	})
}

func collectLocalIPv4Addresses() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	addresses := make([]string, 0, 4)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, rawAddr := range addrs {
			var ip net.IP
			switch value := rawAddr.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}

			if ip == nil {
				continue
			}

			ipv4 := ip.To4()
			if ipv4 == nil || ipv4.IsLoopback() || ipv4.IsLinkLocalUnicast() {
				continue
			}

			candidate := ipv4.String()
			if _, exists := seen[candidate]; exists {
				continue
			}
			seen[candidate] = struct{}{}
			addresses = append(addresses, candidate)
		}
	}

	sort.Strings(addresses)
	return addresses
}

func isPermissionDeniedError(err error) bool {
	if err == nil {
		return false
	}

	return os.IsPermission(err) ||
		strings.Contains(strings.ToLower(err.Error()), "access is denied") ||
		strings.Contains(strings.ToLower(err.Error()), "permission denied") ||
		strings.Contains(strings.ToLower(err.Error()), "user name or password is incorrect") ||
		strings.Contains(strings.ToLower(err.Error()), "specified network password is not correct") ||
		strings.Contains(strings.ToLower(err.Error()), "logon failure") ||
		strings.Contains(strings.ToLower(err.Error()), "unknown user name or bad password") ||
		strings.Contains(strings.ToLower(err.Error()), "the network password is not correct")
}

func formatDirectoryAccessDeniedMessage(path string, err error) string {
	message := fmt.Sprintf("access denied reading %s: %v", path, err)
	if strings.HasPrefix(strings.TrimSpace(path), `\\`) {
		return message + " (UNC browsing and scanning use the backend Windows identity; LDAP credentials are only applied to directory expansion)"
	}

	return message
}
