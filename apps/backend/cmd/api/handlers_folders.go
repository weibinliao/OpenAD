package main

import (
	"net/http"
	"sort"
	"strings"

	"github.com/weibinliao/OpenAD/internal/database"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// folderChild is one immediate subfolder of a scanned path, derived from the
// distinct permission row paths of a scan session. It powers lazy expansion
// of the shares branch in the unified explorer tree.
type folderChild struct {
	Path            string `json:"path"`
	Name            string `json:"name"`
	HasChildren     bool   `json:"has_children"`
	PermissionCount int    `json:"permission_count"`
}

// handleListSessionFolders returns the immediate child folders under `parent`
// (default: the session root) for a scan session.
// GET /api/sessions/:id/folders?parent=<path>
func (application *application) handleListSessionFolders(context *gin.Context) {
	if !database.Ready() {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not initialized"})
		return
	}

	sessionID, err := uuid.Parse(context.Param("id"))
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid session id"})
		return
	}

	var session models.ScanSession
	if err := database.DB.First(&session, "id = ?", sessionID).Error; err != nil {
		context.JSON(http.StatusNotFound, gin.H{"error": "scan session not found"})
		return
	}

	parent := strings.TrimRight(strings.TrimSpace(context.Query("parent")), "\\/")
	if parent == "" {
		parent = strings.TrimRight(session.RootPath, "\\/")
	}

	// Pull the distinct paths once; a scan session tops out at tens of
	// thousands of rows, and distinct paths are far fewer.
	var paths []string
	if err := database.DB.Model(&models.Permission{}).
		Where("scan_session_id = ?", sessionID).
		Distinct().
		Pluck("path", &paths).Error; err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	separator := "\\"
	if strings.Contains(session.RootPath, "/") && !strings.Contains(session.RootPath, "\\") {
		separator = "/"
	}

	parentLower := strings.ToLower(parent)
	prefixLower := parentLower + strings.ToLower(separator)

	type childAccumulator struct {
		path        string
		name        string
		hasChildren bool
		permissions int
	}
	children := make(map[string]*childAccumulator)
	permissionCounts := make(map[string]int)

	// Count permission rows per exact path for the direct children.
	var rows []struct {
		Path  string
		Count int
	}
	if err := database.DB.Model(&models.Permission{}).
		Select("path, COUNT(*) as count").
		Where("scan_session_id = ?", sessionID).
		Group("path").
		Scan(&rows).Error; err == nil {
		for _, row := range rows {
			permissionCounts[strings.ToLower(strings.TrimRight(row.Path, "\\/"))] = row.Count
		}
	}

	for _, rawPath := range paths {
		path := strings.TrimRight(rawPath, "\\/")
		lower := strings.ToLower(path)
		if !strings.HasPrefix(lower, prefixLower) {
			continue
		}
		remainder := path[len(parent)+len(separator):]
		segments := strings.SplitN(remainder, separator, 2)
		childName := segments[0]
		if strings.TrimSpace(childName) == "" {
			continue
		}
		childPath := parent + separator + childName
		key := strings.ToLower(childPath)
		accumulator, exists := children[key]
		if !exists {
			accumulator = &childAccumulator{path: childPath, name: childName}
			children[key] = accumulator
		}
		if len(segments) > 1 {
			accumulator.hasChildren = true
		}
	}

	result := make([]folderChild, 0, len(children))
	for key, accumulator := range children {
		result = append(result, folderChild{
			Path:            accumulator.path,
			Name:            accumulator.name,
			HasChildren:     accumulator.hasChildren,
			PermissionCount: permissionCounts[key],
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	context.JSON(http.StatusOK, gin.H{
		"parent":   parent,
		"items":    result,
		"count":    len(result),
		"root":     session.RootPath,
		"session":  session.ID,
		"finished": session.FinishedAt,
	})
}
