package comparisonservice

import (
	"errors"
	"fmt"
	"strings"

	"github.com/weibinliao/OpenAD/internal/comparison"
	"github.com/weibinliao/OpenAD/internal/database"
	"github.com/weibinliao/OpenAD/internal/historyservice"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Request struct {
	BaselineSessionID string
	CurrentSessionID  string
}

type repository interface {
	GetSession(id uuid.UUID) (*models.ScanSession, error)
	ListSessionPermissions(id uuid.UUID) ([]models.Permission, error)
	SaveSessionChanges(sessionID uuid.UUID, changes []models.PermissionChange) error
}

type Service struct {
	repository repository
}

func New() *Service {
	return NewWithRepository(&databaseRepository{})
}

func NewWithRepository(repository repository) *Service {
	if repository == nil {
		repository = &databaseRepository{}
	}

	return &Service{repository: repository}
}

func (service *Service) Compare(request Request) (*comparison.ChangeReport, error) {
	baselineID, err := parseSessionID(request.BaselineSessionID)
	if err != nil {
		return nil, err
	}

	currentID, err := parseSessionID(request.CurrentSessionID)
	if err != nil {
		return nil, err
	}

	baselineSession, err := service.repository.GetSession(baselineID)
	if err != nil {
		return nil, err
	}
	if baselineSession == nil {
		return nil, historyservice.ErrSessionNotFound
	}

	currentSession, err := service.repository.GetSession(currentID)
	if err != nil {
		return nil, err
	}
	if currentSession == nil {
		return nil, historyservice.ErrSessionNotFound
	}

	baselinePermissions, err := service.repository.ListSessionPermissions(baselineID)
	if err != nil {
		return nil, err
	}

	currentPermissions, err := service.repository.ListSessionPermissions(currentID)
	if err != nil {
		return nil, err
	}

	engine := comparison.NewComparisonEngine(baselineSession, currentSession)
	report, err := engine.DetectChanges(baselinePermissions, currentPermissions)
	if err != nil {
		return nil, err
	}

	if err := service.repository.SaveSessionChanges(currentID, report.Changes); err != nil {
		return nil, err
	}

	return report, nil
}

func parseSessionID(value string) (uuid.UUID, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return uuid.Nil, fmt.Errorf("%w: %s", historyservice.ErrInvalidSessionID, trimmed)
	}

	sessionID, err := uuid.Parse(trimmed)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %s", historyservice.ErrInvalidSessionID, trimmed)
	}

	return sessionID, nil
}

type databaseRepository struct{}

func (repository *databaseRepository) GetSession(id uuid.UUID) (*models.ScanSession, error) {
	if !database.Ready() {
		return nil, historyservice.ErrDatabaseUnavailable
	}

	var session models.ScanSession
	if err := database.DB.First(&session, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &session, nil
}

func (repository *databaseRepository) ListSessionPermissions(id uuid.UUID) ([]models.Permission, error) {
	if !database.Ready() {
		return nil, historyservice.ErrDatabaseUnavailable
	}

	permissions := make([]models.Permission, 0)
	if err := database.DB.Where("scan_session_id = ?", id).Find(&permissions).Error; err != nil {
		return nil, err
	}

	return permissions, nil
}

func (repository *databaseRepository) SaveSessionChanges(sessionID uuid.UUID, changes []models.PermissionChange) error {
	if !database.Ready() {
		return historyservice.ErrDatabaseUnavailable
	}

	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("scan_session_id = ?", sessionID).Delete(&models.PermissionChange{}).Error; err != nil {
			return err
		}

		if len(changes) == 0 {
			return nil
		}

		normalized := make([]models.PermissionChange, 0, len(changes))
		for _, change := range changes {
			change.ScanSessionID = sessionID
			normalized = append(normalized, change)
		}

		return tx.CreateInBatches(normalized, 500).Error
	})
}
