package main

import (
	"errors"
	"strings"
)

var errUNCServerShareDiscoveryUnsupported = errors.New("UNC server share discovery is only supported on Windows")

func normalizeUNCPath(path string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(path, "/", `\`))
	return strings.TrimRight(normalized, `\`)
}

func isUNCPath(path string) bool {
	return strings.HasPrefix(normalizeUNCPath(path), `\\`)
}

func uncPathParts(path string) []string {
	normalized := normalizeUNCPath(path)
	if !strings.HasPrefix(normalized, `\\`) {
		return nil
	}

	parts := strings.Split(strings.TrimLeft(normalized, `\`), `\`)
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}

	return filtered
}

func isUNCServerRootPath(path string) bool {
	return len(uncPathParts(path)) == 1
}

func uncServerRootPath(path string) (string, error) {
	parts := uncPathParts(path)
	if len(parts) < 1 {
		return "", errors.New("UNC path must include a server name")
	}

	return `\\` + parts[0], nil
}

func uncShareRootPath(path string) (string, error) {
	parts := uncPathParts(path)
	if len(parts) < 2 {
		return "", errors.New("UNC path must include a share name")
	}

	return `\\` + parts[0] + `\` + parts[1], nil
}
