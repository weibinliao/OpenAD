package scanservice

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/weibinliao/OpenAD/internal/database"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/scanner"
	"github.com/google/uuid"
)

type Request struct {
	ScanID                      string
	Path                        string
	MaxDepth                    int
	IncludeInherited            bool
	Progress                    ProgressCallback
	Context                     context.Context
	EffectivePermissionExpander EffectivePermissionExpander
}

type EffectivePermissionExpander interface {
	Expand(ctx context.Context, permissions []scanner.Permission) ([]scanner.Permission, error)
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
	SessionID        string               `json:"session_id,omitempty"`
	RootPath         string               `json:"root_path"`
	MaxDepth         int                  `json:"max_depth"`
	IncludeInherited bool                 `json:"include_inherited"`
	ItemsScanned     int                  `json:"items_scanned"`
	PermissionCount  int                  `json:"permission_count"`
	StartedAt        time.Time            `json:"started_at"`
	FinishedAt       time.Time            `json:"finished_at"`
	Permissions      []scanner.Permission `json:"permissions"`
	Skipped          []scanner.PathError  `json:"skipped,omitempty"`
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
}

func New() *Service {
	return NewWithDependencies(scanner.NewNTFSScanner(), &databaseSessionRepository{})
}

func NewWithDependencies(directoryScanner directoryScanner, repository sessionRepository) *Service {
	if directoryScanner == nil {
		directoryScanner = scanner.NewNTFSScanner()
	}

	if repository == nil {
		repository = &databaseSessionRepository{}
	}

	return &Service{
		scanner:    directoryScanner,
		repository: repository,
	}
}

func (service *Service) Run(request Request) (*Response, error) {
	startedAt := time.Now().UTC()
	if closer, ok := request.EffectivePermissionExpander.(interface{ Close() }); ok {
		defer closer.Close()
	}

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

		permissions, err = request.EffectivePermissionExpander.Expand(ctx, permissions)
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
	}

	finishedAt := time.Now().UTC()
	response := &Response{
		RootPath:         result.RootPath,
		MaxDepth:         result.MaxDepth,
		IncludeInherited: result.IncludeInherited,
		ItemsScanned:     result.ItemsScanned,
		PermissionCount:  len(permissions),
		StartedAt:        startedAt,
		FinishedAt:       finishedAt,
		Permissions:      permissions,
		Skipped:          result.Skipped,
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
		"status":           "completed",
		"items_scanned":    response.ItemsScanned,
		"permission_count": response.PermissionCount,
		"finished_at":      &finishedAt,
		"error_message":    "",
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
