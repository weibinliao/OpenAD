package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weibinliao/OpenAD/internal/database"
	"github.com/weibinliao/OpenAD/internal/identityresolution"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/scanservice"
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

type effectivePermissionPreparationError struct {
	err error
}

func (err *effectivePermissionPreparationError) Error() string {
	return fmt.Sprintf("failed to prepare effective permission expansion: %v", err.err)
}

func (err *effectivePermissionPreparationError) Unwrap() error {
	return err.err
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

	if isUNCServerRootPath(request.Path) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "select a UNC share or subdirectory before scanning"})
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
	registration, err := application.scanCancels.register(scanID, cancelScan)
	if errors.Is(err, errScanIDAlreadyActive) {
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	removeRegistrationOnReturn := true
	defer func() {
		if removeRegistrationOnReturn {
			application.scanCancels.remove(scanID, registration)
		}
	}()

	requestContext := ctx.Request.Context()
	go func() {
		select {
		case <-requestContext.Done():
			cancelScan()
		case <-scanContext.Done():
		}
	}()

	var expanderFactory func() (scanservice.EffectivePermissionExpander, error)
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

		effectivePermissions := *request.EffectivePermissions
		snapshotConnectionID := snapshotConnectionIDForRequest(effectivePermissions)
		expanderFactory = func() (scanservice.EffectivePermissionExpander, error) {
			liveExpander, factoryErr := application.ad.NewEffectivePermissionExpander(
				effectivePermissions.Server,
				effectivePermissions.BaseDN,
				effectivePermissions.Username,
				effectivePermissions.Password,
				effectivePermissions.ExcludeGroupPatterns,
				effectivePermissions.ExcludeUserPatterns,
			)
			return identityresolution.NewService(database.DB, identityresolution.Options{
				ConnectionID:    snapshotConnectionID,
				LiveExpander:    liveExpander,
				LiveUnavailable: factoryErr != nil,
			}), nil
		}
	}

	resultChannel := make(chan *scanservice.Response, 1)
	errorChannel := make(chan error, 1)
	scanFinished := make(chan struct{})

	go func() {
		result, err := application.scans.Run(scanservice.Request{
			ScanID:                             scanID,
			Path:                               request.Path,
			MaxDepth:                           depth,
			IncludeInherited:                   includeInherited,
			Progress:                           application.buildScanProgressCallback(scanID),
			Context:                            scanContext,
			EffectivePermissionExpanderFactory: expanderFactory,
		})
		close(scanFinished)
		if err != nil {
			errorChannel <- err
			return
		}

		resultChannel <- result
	}()

	select {
	case err := <-errorChannel:
		var preparationErr *effectivePermissionPreparationError
		if errors.Is(err, context.Canceled) {
			ctx.JSON(http.StatusOK, gin.H{
				"scan_id": scanID,
				"status":  "cancelled",
				"message": "scan cancelled",
			})
			return
		}
		if errors.As(err, &preparationErr) {
			ctx.JSON(http.StatusBadGateway, gin.H{"error": preparationErr.Error()})
			return
		}
		if errors.Is(err, scanservice.ErrScanConcurrencyLimitReached) {
			ctx.JSON(http.StatusConflict, gin.H{
				"error": "maximum concurrent scans reached; wait for the active scan to finish or cancel it before retrying",
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	case result := <-resultChannel:
		ctx.JSON(http.StatusOK, result)
		return
	case <-ctx.Request.Context().Done():
		cancelScan()
		removeRegistrationOnReturn = false
		go func() {
			<-scanFinished
			application.scanCancels.remove(scanID, registration)
		}()
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

func snapshotConnectionIDForRequest(request EffectivePermissionRequest) uuid.UUID {
	if value := strings.TrimSpace(request.ConnectionID); value != "" {
		if id, err := uuid.Parse(value); err == nil {
			return id
		}
		return uuid.Nil
	}
	if !database.Ready() {
		return uuid.Nil
	}

	server := strings.ToLower(strings.TrimSpace(request.Server))
	baseDN := strings.ToLower(strings.TrimSpace(request.BaseDN))
	if server == "" || baseDN == "" {
		return uuid.Nil
	}

	var profiles []models.ADConnectionProfile
	if err := database.DB.
		Where("LOWER(server) = ? AND LOWER(base_dn) = ?", server, baseDN).
		Limit(2).
		Find(&profiles).Error; err != nil || len(profiles) != 1 {
		return uuid.Nil
	}

	return profiles[0].ID
}
