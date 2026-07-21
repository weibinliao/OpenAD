package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/weibinliao/OpenAD/internal/database"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func initDatabase() error {
	return database.InitFromEnv()
}

func loadSessionAndPermissions(sessionID string) (*models.ScanSession, []models.Permission, error) {
	parsedID, err := uuid.Parse(strings.TrimSpace(sessionID))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid session id %q: %w", strings.TrimSpace(sessionID), err)
	}

	if !database.Ready() {
		return nil, nil, errors.New("database is not initialized")
	}

	var session models.ScanSession
	if err := database.DB.First(&session, "id = ?", parsedID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, fmt.Errorf("scan session %s not found", parsedID)
		}

		return nil, nil, err
	}

	permissions := make([]models.Permission, 0)
	if err := database.DB.
		Where("scan_session_id = ?", parsedID).
		Order("path ASC, trustee ASC, created_at ASC").
		Find(&permissions).Error; err != nil {
		return nil, nil, err
	}

	return &session, permissions, nil
}
