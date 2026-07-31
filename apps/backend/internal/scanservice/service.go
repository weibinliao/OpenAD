package scanservice

import (
	"context"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/weibinliao/OpenAD/internal/database"
	"github.com/weibinliao/OpenAD/internal/identityresolution"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/scanner"
)

type Request struct {
	ScanID                             string
	Path                               string
	MaxDepth                           int
	IncludeInherited                   bool
	Progress                           ProgressCallback
	Context                            context.Context
	EffectivePermissionExpander        EffectivePermissionExpander
	EffectivePermissionExpanderFactory func() (EffectivePermissionExpander, error)
}

type EffectivePermissionExpander interface {
	Expand(ctx context.Context, permissions []scanner.Permission) ([]scanner.Permission, error)
}

type identityResolutionMetadataProvider interface {
	Metadata() identityresolution.Metadata
}

type ProgressEvent struct {
	ScanID          string `json:"scan_id,omitempty"`
	SessionID       string `json:"session_id,omitempty"`
	ItemsScanned    int    `json:"items_scanned"`
	PermissionCount int    `json:"permission_count"`
	CurrentPath     string `json:"current_path,omitempty"`
	Status          string `json:"status"`
	Error           string `json:"error,omitempty"`
}

type ProgressCallback func(ProgressEvent)

type Response struct {
	SessionID          string                      `json:"session_id,omitempty"`
	RootPath           string                      `json:"root_path"`
	MaxDepth           int                         `json:"max_depth"`
	IncludeInherited   bool                        `json:"include_inherited"`
	ItemsScanned       int                         `json:"items_scanned"`
	PermissionCount    int                         `json:"permission_count"`
	StartedAt          time.Time                   `json:"started_at"`
	FinishedAt         time.Time                   `json:"finished_at"`
	Permissions        []scanner.Permission        `json:"permissions"`
	Skipped            []scanner.PathError         `json:"skipped,omitempty"`
	IdentityResolution identityresolution.Metadata `json:"identity_resolution"`
}

type directoryScanner interface {
	ScanDirectory(path string, options scanner.Options) (*scanner.Result, error)
}

type sessionRepository interface {
	CreateSession(session *models.ScanSession) error
	CompleteSession(session *models.ScanSession, response *Response) error
	FailSession(session *models.ScanSession, scanErr error) error
	CancelSession(session *models.ScanSession) error
}

type Service struct {
	scanner    directoryScanner
	repository sessionRepository
	scanSlots  chan struct{}
}

const defaultMaxConcurrentScans = 1

var ErrScanConcurrencyLimitReached = errors.New("maximum concurrent scans reached")

func New() *Service {
	return newService(scanner.NewNTFSScanner(), &databaseSessionRepository{}, maxConcurrentScansFromEnv())
}

func NewWithDependencies(directoryScanner directoryScanner, repository sessionRepository) *Service {
	return newService(directoryScanner, repository, defaultMaxConcurrentScans)
}

func newService(directoryScanner directoryScanner, repository sessionRepository, maxConcurrentScans int) *Service {
	if directoryScanner == nil {
		directoryScanner = scanner.NewNTFSScanner()
	}

	if repository == nil {
		repository = &databaseSessionRepository{}
	}
	if maxConcurrentScans < 1 {
		maxConcurrentScans = defaultMaxConcurrentScans
	}

	return &Service{
		scanner:    directoryScanner,
		repository: repository,
		scanSlots:  make(chan struct{}, maxConcurrentScans),
	}
}

func (service *Service) Run(request Request) (*Response, error) {
	if !service.tryAcquireScanSlot() {
		return nil, ErrScanConcurrencyLimitReached
	}
	defer service.releaseScanSlot()

	startedAt := time.Now().UTC()

	session := service.createSession(request, startedAt)
	sessionID := sessionID(session)

	service.emitProgress(request, ProgressEvent{
		SessionID:   sessionID,
		CurrentPath: request.Path,
		Status:      "running",
	})

	result, err := service.scanner.ScanDirectory(request.Path, scanner.Options{
		MaxDepth:         request.MaxDepth,
		IncludeInherited: request.IncludeInherited,
		Context:          request.Context,
		Progress: func(progress scanner.Progress) {
			service.emitProgress(request, ProgressEvent{
				SessionID:       sessionID,
				ItemsScanned:    progress.ItemsScanned,
				PermissionCount: progress.PermissionCount,
				CurrentPath:     progress.CurrentPath,
				Status:          "running",
			})
		},
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			service.cancelSession(session)
			service.emitProgress(request, ProgressEvent{
				SessionID:   sessionID,
				CurrentPath: request.Path,
				Status:      "cancelled",
				Error:       "scan cancelled",
			})
			return nil, err
		}

		service.failSession(session, err)
		service.emitProgress(request, ProgressEvent{
			SessionID:   sessionID,
			CurrentPath: request.Path,
			Status:      "failed",
			Error:       err.Error(),
		})
		return nil, err
	}

	permissions := result.Permissions
	identityMetadata := rawIdentityMetadata(permissions, "raw")
	var preparationErr error
	if request.EffectivePermissionExpander == nil && request.EffectivePermissionExpanderFactory != nil {
		request.EffectivePermissionExpander, preparationErr = request.EffectivePermissionExpanderFactory()
	}
	if closer, ok := request.EffectivePermissionExpander.(interface{ Close() }); ok {
		defer closer.Close()
	}
	if preparationErr != nil {
		permissions = fallbackPermissions(permissions, "resolver_error")
		identityMetadata = rawIdentityMetadata(permissions, "raw-fallback")
		identityMetadata.Warning = "identity resolution unavailable"
	}
	if request.EffectivePermissionExpander != nil {
		ctx := request.Context
		if ctx == nil {
			ctx = context.Background()
		}

		service.emitProgress(request, ProgressEvent{
			SessionID:       sessionID,
			ItemsScanned:    result.ItemsScanned,
			PermissionCount: len(permissions),
			CurrentPath:     request.Path,
			Status:          "expanding",
		})

		expandedPermissions, expandErr := request.EffectivePermissionExpander.Expand(ctx, permissions)
		if expandErr != nil {
			if errors.Is(expandErr, context.Canceled) {
				service.cancelSession(session)
				service.emitProgress(request, ProgressEvent{
					SessionID:   sessionID,
					CurrentPath: request.Path,
					Status:      "cancelled",
					Error:       "scan cancelled",
				})
				return nil, expandErr
			}

			permissions = fallbackPermissions(permissions, "resolver_error")
			identityMetadata = rawIdentityMetadata(permissions, "raw-fallback")
			identityMetadata.Warning = "identity resolution unavailable"
		} else if len(permissions) > 0 && len(expandedPermissions) == 0 {
			permissions = fallbackPermissions(permissions, "empty_result")
			identityMetadata = rawIdentityMetadata(permissions, "raw-fallback")
			identityMetadata.Warning = "identity resolution returned no principals"
		} else {
			permissions = expandedPermissions
			if provider, ok := request.EffectivePermissionExpander.(identityResolutionMetadataProvider); ok {
				identityMetadata = provider.Metadata()
			} else {
				identityMetadata = resolvedIdentityMetadata(result.Permissions, "ldap")
			}
		}
	}

	finishedAt := time.Now().UTC()
	response := &Response{
		RootPath:           result.RootPath,
		MaxDepth:           result.MaxDepth,
		IncludeInherited:   result.IncludeInherited,
		ItemsScanned:       result.ItemsScanned,
		PermissionCount:    len(permissions),
		StartedAt:          startedAt,
		FinishedAt:         finishedAt,
		Permissions:        permissions,
		Skipped:            result.Skipped,
		IdentityResolution: identityMetadata,
	}

	if session != nil {
		response.SessionID = session.ID.String()
	}

	service.completeSession(session, response)
	service.emitProgress(request, ProgressEvent{
		SessionID:       sessionID,
		ItemsScanned:    response.ItemsScanned,
		PermissionCount: response.PermissionCount,
		CurrentPath:     response.RootPath,
		Status:          "completed",
	})

	return response, nil
}

func fallbackPermissions(permissions []scanner.Permission, reason string) []scanner.Permission {
	result := make([]scanner.Permission, 0, len(permissions))
	for _, permission := range permissions {
		permission.ResolutionSource = "raw"
		permission.ResolutionReason = reason
		result = append(result, permission)
	}
	return result
}

func rawIdentityMetadata(permissions []scanner.Permission, mode string) identityresolution.Metadata {
	return identityresolution.Metadata{
		Mode:                     mode,
		UnresolvedPrincipalCount: countDistinctPrincipals(permissions),
	}
}

func resolvedIdentityMetadata(permissions []scanner.Permission, mode string) identityresolution.Metadata {
	return identityresolution.Metadata{
		Mode:                   mode,
		ResolvedPrincipalCount: countDistinctPrincipals(permissions),
	}
}

func countDistinctPrincipals(permissions []scanner.Permission) int {
	principals := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		principal := strings.TrimSpace(permission.TrusteeSID)
		if principal == "" {
			principal = strings.TrimSpace(permission.Trustee)
		}
		if principal == "" {
			continue
		}
		principals[strings.ToUpper(principal)] = struct{}{}
	}
	return len(principals)
}

func (service *Service) tryAcquireScanSlot() bool {
	if service.scanSlots == nil {
		return true
	}

	select {
	case service.scanSlots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (service *Service) releaseScanSlot() {
	if service.scanSlots != nil {
		<-service.scanSlots
	}
}

func maxConcurrentScansFromEnv() int {
	for _, key := range []string{"PERMISSION_PROTECTOR_MAX_CONCURRENT_SCANS", "FSA_MAX_CONCURRENT_SCANS"} {
		rawValue := strings.TrimSpace(os.Getenv(key))
		if rawValue == "" {
			continue
		}

		value, err := strconv.Atoi(rawValue)
		if err != nil || value < 1 {
			log.Printf("invalid %s=%q; using default maximum concurrent scans: %d", key, rawValue, defaultMaxConcurrentScans)
			return defaultMaxConcurrentScans
		}
		return value
	}

	return defaultMaxConcurrentScans
}

func (service *Service) emitProgress(request Request, event ProgressEvent) {
	if request.Progress == nil {
		return
	}

	if event.ScanID == "" {
		event.ScanID = request.ScanID
	}

	request.Progress(event)
}

func sessionID(session *models.ScanSession) string {
	if session == nil {
		return ""
	}

	return session.ID.String()
}

func (service *Service) createSession(request Request, startedAt time.Time) *models.ScanSession {
	session := &models.ScanSession{
		RootPath:         request.Path,
		Status:           "running",
		MaxDepth:         request.MaxDepth,
		IncludeInherited: request.IncludeInherited,
		StartedAt:        startedAt,
	}

	if service.repository == nil {
		return nil
	}

	if err := service.repository.CreateSession(session); err != nil {
		log.Printf("failed to create scan session: %v", err)
		return nil
	}

	if session.ID == uuid.Nil {
		return nil
	}

	return session
}

func (service *Service) completeSession(session *models.ScanSession, response *Response) {
	if session == nil || service.repository == nil {
		return
	}

	if err := service.repository.CompleteSession(session, response); err != nil {
		log.Printf("failed to complete scan session %s: %v", session.ID, err)
	}
}

func (service *Service) failSession(session *models.ScanSession, scanErr error) {
	if session == nil || service.repository == nil {
		return
	}

	if err := service.repository.FailSession(session, scanErr); err != nil {
		log.Printf("failed to mark scan session %s as failed: %v", session.ID, err)
	}
}

func (service *Service) cancelSession(session *models.ScanSession) {
	if session == nil || service.repository == nil {
		return
	}

	if err := service.repository.CancelSession(session); err != nil {
		log.Printf("failed to mark scan session %s as cancelled: %v", session.ID, err)
	}
}

type databaseSessionRepository struct{}

func (repository *databaseSessionRepository) CreateSession(session *models.ScanSession) error {
	if !database.Ready() {
		return nil
	}

	return database.DB.Create(session).Error
}

func (repository *databaseSessionRepository) CompleteSession(session *models.ScanSession, response *Response) error {
	if !database.Ready() {
		return nil
	}

	finishedAt := response.FinishedAt
	updates := map[string]any{
		"status":                      "completed",
		"items_scanned":               response.ItemsScanned,
		"permission_count":            response.PermissionCount,
		"finished_at":                 &finishedAt,
		"error_message":               "",
		"identity_resolution_mode":    response.IdentityResolution.Mode,
		"resolved_principal_count":    response.IdentityResolution.ResolvedPrincipalCount,
		"unresolved_principal_count":  response.IdentityResolution.UnresolvedPrincipalCount,
		"identity_resolution_warning": response.IdentityResolution.Warning,
	}
	if response.IdentityResolution.DirectorySyncRunID != uuid.Nil {
		runID := response.IdentityResolution.DirectorySyncRunID
		updates["directory_sync_run_id"] = &runID
	}

	if err := database.DB.Model(session).Updates(updates).Error; err != nil {
		return err
	}

	permissions := make([]models.Permission, 0, len(response.Permissions))
	for _, permission := range response.Permissions {
		permissions = append(permissions, models.Permission{
			ScanSessionID:             session.ID,
			Path:                      permission.Path,
			Trustee:                   permission.Trustee,
			TrusteeSID:                permission.TrusteeSID,
			Rights:                    permission.Rights,
			Type:                      permission.Type,
			Inherited:                 permission.Inherited,
			Source:                    permission.Source,
			AppliesTo:                 permission.AppliesTo,
			AccountType:               permission.AccountType,
			AccessMask:                permission.AccessMask,
			RiskLevel:                 permission.RiskLevel,
			ParentDelta:               permission.ParentDelta,
			AccountName:               permission.AccountName,
			FirstName:                 permission.FirstName,
			LastName:                  permission.LastName,
			Email:                     permission.Email,
			Department:                permission.Department,
			Division:                  permission.Division,
			Domain:                    permission.Domain,
			OriginatingGroup:          permission.OriginatingGroup,
			GroupInheritanceHierarchy: permission.GroupInheritanceHierarchy,
			ResolutionSource:          permission.ResolutionSource,
			ResolutionReason:          permission.ResolutionReason,
		})
	}

	if len(permissions) == 0 {
		return nil
	}

	return database.DB.CreateInBatches(permissions, 500).Error
}

func (repository *databaseSessionRepository) FailSession(session *models.ScanSession, scanErr error) error {
	if !database.Ready() {
		return nil
	}

	finishedAt := time.Now().UTC()
	updates := map[string]any{
		"status":        "failed",
		"finished_at":   &finishedAt,
		"error_message": scanErr.Error(),
	}

	return database.DB.Model(session).Updates(updates).Error
}

func (repository *databaseSessionRepository) CancelSession(session *models.ScanSession) error {
	if !database.Ready() {
		return nil
	}

	finishedAt := time.Now().UTC()
	updates := map[string]any{
		"status":        "cancelled",
		"finished_at":   &finishedAt,
		"error_message": "scan cancelled",
	}

	return database.DB.Model(session).Updates(updates).Error
}
