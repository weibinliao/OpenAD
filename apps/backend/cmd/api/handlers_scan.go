package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/scanservice"
	"github.com/gin-gonic/gin"
)

type ScanRequest struct {
	Path                 string                      `json:"path" binding:"required"`
	Depth                int                         `json:"depth"`
	IncludeInherited     *bool                       `json:"include_inherited"`
	ScanID               string                      `json:"scan_id"`
	EffectivePermissions *EffectivePermissionRequest `json:"effective_permissions"`
}

type EffectivePermissionRequest struct {
	Enabled              bool     `json:"enabled"`
	ConnectionID         string   `json:"connection_id"`
	Server               string   `json:"server"`
	BaseDN               string   `json:"base_dn"`
	Username             string   `json:"username"`
	Password             string   `json:"password"`
	ExcludeGroupPatterns []string `json:"exclude_group_patterns"`
	ExcludeUserPatterns  []string `json:"exclude_user_patterns"`
}

type PermissionConflictRequest struct {
	Permissions []models.Permission `json:"permissions" binding:"required"`
}

func (application *application) handleScan(ctx *gin.Context) {
	var request ScanRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	includeInherited := true
	if request.IncludeInherited != nil {
		includeInherited = *request.IncludeInherited
	}

	depth := request.Depth
	if depth == 0 {
		depth = -1
	}

	scanID := strings.TrimSpace(request.ScanID)
	scanContext, cancelScan := context.WithCancel(context.Background())
	defer cancelScan()
	application.scanCancels.register(scanID, cancelScan)
	defer application.scanCancels.remove(scanID)

	requestContext := ctx.Request.Context()
	go func() {
		select {
		case <-requestContext.Done():
			cancelScan()
		case <-scanContext.Done():
		}
	}()

	var err error
	var expander scanservice.EffectivePermissionExpander
	if shouldUseEffectivePermissionExpansion(request.EffectivePermissions) {
		// Resolve a stored connection_id into inline credentials when supplied,
		// so callers can trigger AD-aware scans without re-entering credentials.
		if strings.TrimSpace(request.EffectivePermissions.ConnectionID) != "" {
			server, baseDN, username, password, resolveErr := resolveADCredentials(
				request.EffectivePermissions.ConnectionID,
				request.EffectivePermissions.Server,
				request.EffectivePermissions.BaseDN,
				request.EffectivePermissions.Username,
				request.EffectivePermissions.Password,
			)
			if resolveErr != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": resolveErr.Error()})
				return
			}
			request.EffectivePermissions.Server = server
			request.EffectivePermissions.BaseDN = baseDN
			request.EffectivePermissions.Username = username
			request.EffectivePermissions.Password = password
		}

		if err := validateEffectivePermissionRequest(*request.EffectivePermissions); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		expander, err = application.ad.NewEffectivePermissionExpander(
			request.EffectivePermissions.Server,
			request.EffectivePermissions.BaseDN,
			request.EffectivePermissions.Username,
			request.EffectivePermissions.Password,
			request.EffectivePermissions.ExcludeGroupPatterns,
			request.EffectivePermissions.ExcludeUserPatterns,
		)
		if err != nil {
			ctx.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to prepare effective permission expansion: %v", err)})
			return
		}
	}

	resultChannel := make(chan *scanservice.Response, 1)
	errorChannel := make(chan error, 1)

	go func() {
		result, err := application.scans.Run(scanservice.Request{
			ScanID:                      scanID,
			Path:                        request.Path,
			MaxDepth:                    depth,
			IncludeInherited:            includeInherited,
			Progress:                    application.buildScanProgressCallback(scanID),
			Context:                     scanContext,
			EffectivePermissionExpander: expander,
		})
		if err != nil {
			errorChannel <- err
			return
		}

		resultChannel <- result
	}()

	select {
	case err := <-errorChannel:
		if errors.Is(err, context.Canceled) {
			ctx.JSON(http.StatusOK, gin.H{
				"scan_id": scanID,
				"status":  "cancelled",
				"message": "scan cancelled",
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	case result := <-resultChannel:
		ctx.JSON(http.StatusOK, result)
		return
	case <-ctx.Request.Context().Done():
		return
	}
}

func (application *application) handlePermissionConflicts(context *gin.Context) {
	var request PermissionConflictRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conflicts := buildPermissionConflicts(request.Permissions)
	context.JSON(http.StatusOK, gin.H{
		"count":     len(conflicts),
		"conflicts": conflicts,
	})
}

type permissionConflict struct {
	Path           string   `json:"path"`
	Trustee        string   `json:"trustee"`
	AllowRights    []string `json:"allow_rights"`
	DenyRights     []string `json:"deny_rights"`
	HasExplicit    bool     `json:"has_explicit"`
	HasInherited   bool     `json:"has_inherited"`
	PrecedenceNote string   `json:"precedence_note"`
}

func buildPermissionConflicts(permissions []models.Permission) []permissionConflict {
	type bucket struct {
		path         string
		trustee      string
		allows       map[string]struct{}
		denies       map[string]struct{}
		hasExplicit  bool
		hasInherited bool
	}

	index := make(map[string]*bucket)
	for _, permission := range permissions {
		path := strings.TrimSpace(permission.Path)
		trustee := strings.TrimSpace(permission.Trustee)
		if path == "" || trustee == "" {
			continue
		}

		key := strings.ToLower(path) + "::" + strings.ToLower(trustee)
		current, ok := index[key]
		if !ok {
			current = &bucket{
				path:    path,
				trustee: trustee,
				allows:  make(map[string]struct{}),
				denies:  make(map[string]struct{}),
			}
			index[key] = current
		}

		right := strings.TrimSpace(permission.Rights)
		if strings.EqualFold(strings.TrimSpace(permission.Type), "deny") {
			current.denies[right] = struct{}{}
		} else {
			current.allows[right] = struct{}{}
		}
		if permission.Inherited {
			current.hasInherited = true
		} else {
			current.hasExplicit = true
		}
	}

	conflicts := make([]permissionConflict, 0)
	for _, current := range index {
		if len(current.allows) == 0 || len(current.denies) == 0 {
			continue
		}

		allowRights := mapKeysSorted(current.allows)
		denyRights := mapKeysSorted(current.denies)
		note := "Deny entries are evaluated before allow entries for overlapping rights."
		if current.hasExplicit && current.hasInherited {
			note += " Explicit entries override inherited entries at the same object."
		}

		conflicts = append(conflicts, permissionConflict{
			Path:           current.path,
			Trustee:        current.trustee,
			AllowRights:    allowRights,
			DenyRights:     denyRights,
			HasExplicit:    current.hasExplicit,
			HasInherited:   current.hasInherited,
			PrecedenceNote: note,
		})
	}

	sort.Slice(conflicts, func(i, j int) bool {
		if strings.EqualFold(conflicts[i].Path, conflicts[j].Path) {
			return strings.ToLower(conflicts[i].Trustee) < strings.ToLower(conflicts[j].Trustee)
		}
		return strings.ToLower(conflicts[i].Path) < strings.ToLower(conflicts[j].Path)
	})

	return conflicts
}

func mapKeysSorted(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Slice(keys, func(i, j int) bool {
		return strings.ToLower(keys[i]) < strings.ToLower(keys[j])
	})

	return keys
}

func validateEffectivePermissionRequest(request EffectivePermissionRequest) error {
	requiredFields := map[string]string{
		"server":   request.Server,
		"base_dn":  request.BaseDN,
		"username": request.Username,
		"password": request.Password,
	}

	for field, value := range requiredFields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("effective_permissions.%s is required when enabled", field)
		}
	}

	return nil
}

func shouldUseEffectivePermissionExpansion(request *EffectivePermissionRequest) bool {
	if request == nil {
		return false
	}

	if request.Enabled {
		return true
	}

	if strings.TrimSpace(request.ConnectionID) != "" {
		return true
	}

	return strings.TrimSpace(request.Server) != "" &&
		strings.TrimSpace(request.BaseDN) != "" &&
		strings.TrimSpace(request.Username) != "" &&
		strings.TrimSpace(request.Password) != ""
}
