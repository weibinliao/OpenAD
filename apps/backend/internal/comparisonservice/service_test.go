package comparisonservice

import (
	"errors"
	"testing"

	"github.com/weibinliao/OpenAD/internal/historyservice"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComparePersistsDetectedChanges(t *testing.T) {
	baselineSession := models.ScanSession{ID: uuid.New(), RootPath: `C:\Finance`, Status: "completed"}
	currentSession := models.ScanSession{ID: uuid.New(), RootPath: `C:\Finance`, Status: "completed"}
	repository := &stubComparisonRepository{
		sessions: map[uuid.UUID]*models.ScanSession{
			baselineSession.ID: &baselineSession,
			currentSession.ID:  &currentSession,
		},
		permissionsBySessionID: map[uuid.UUID][]models.Permission{
			baselineSession.ID: []models.Permission{},
			currentSession.ID: {
				{
					Path:       `C:\Finance`,
					Trustee:    `DOMAIN\Alice`,
					TrusteeSID: "S-1-5-21-100",
					Rights:     "Read",
					Type:       "Allow",
				},
			},
		},
	}

	service := NewWithRepository(repository)
	report, err := service.Compare(Request{
		BaselineSessionID: baselineSession.ID.String(),
		CurrentSessionID:  currentSession.ID.String(),
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, currentSession.ID.String(), report.CurrentID)
	assert.Len(t, report.Changes, 1)
	assert.Equal(t, currentSession.ID, repository.savedSessionID)
	require.Len(t, repository.savedChanges, 1)
	assert.Equal(t, report.Changes[0].ChangeType, repository.savedChanges[0].ChangeType)
	assert.Equal(t, report.Changes[0].Path, repository.savedChanges[0].Path)
}

func TestCompareReturnsValidationAndLookupErrors(t *testing.T) {
	t.Run("invalid session id", func(t *testing.T) {
		service := NewWithRepository(&stubComparisonRepository{})

		report, err := service.Compare(Request{
			BaselineSessionID: "not-a-uuid",
			CurrentSessionID:  uuid.NewString(),
		})

		assert.Nil(t, report)
		assert.ErrorIs(t, err, historyservice.ErrInvalidSessionID)
	})

	t.Run("session not found", func(t *testing.T) {
		service := NewWithRepository(&stubComparisonRepository{})

		report, err := service.Compare(Request{
			BaselineSessionID: uuid.NewString(),
			CurrentSessionID:  uuid.NewString(),
		})

		assert.Nil(t, report)
		assert.ErrorIs(t, err, historyservice.ErrSessionNotFound)
	})

	t.Run("repository error", func(t *testing.T) {
		repositoryErr := errors.New("repository unavailable")
		service := NewWithRepository(&stubComparisonRepository{getSessionErr: repositoryErr})

		report, err := service.Compare(Request{
			BaselineSessionID: uuid.NewString(),
			CurrentSessionID:  uuid.NewString(),
		})

		assert.Nil(t, report)
		assert.Equal(t, repositoryErr, err)
	})
}

func TestDatabaseRepositoryReturnsDatabaseUnavailableWhenDatabaseIsNotReady(t *testing.T) {
	repository := &databaseRepository{}

	session, err := repository.GetSession(uuid.New())
	assert.Nil(t, session)
	assert.ErrorIs(t, err, historyservice.ErrDatabaseUnavailable)

	permissions, err := repository.ListSessionPermissions(uuid.New())
	assert.Nil(t, permissions)
	assert.ErrorIs(t, err, historyservice.ErrDatabaseUnavailable)

	err = repository.SaveSessionChanges(uuid.New(), nil)
	assert.ErrorIs(t, err, historyservice.ErrDatabaseUnavailable)
}

type stubComparisonRepository struct {
	sessions               map[uuid.UUID]*models.ScanSession
	permissionsBySessionID map[uuid.UUID][]models.Permission
	getSessionErr          error
	listPermissionsErr     error
	saveChangesErr         error
	savedSessionID         uuid.UUID
	savedChanges           []models.PermissionChange
}

func (repository *stubComparisonRepository) GetSession(id uuid.UUID) (*models.ScanSession, error) {
	if repository.getSessionErr != nil {
		return nil, repository.getSessionErr
	}

	session := repository.sessions[id]
	if session == nil {
		return nil, nil
	}

	cloned := *session
	return &cloned, nil
}

func (repository *stubComparisonRepository) ListSessionPermissions(id uuid.UUID) ([]models.Permission, error) {
	if repository.listPermissionsErr != nil {
		return nil, repository.listPermissionsErr
	}

	return append([]models.Permission(nil), repository.permissionsBySessionID[id]...), nil
}

func (repository *stubComparisonRepository) SaveSessionChanges(sessionID uuid.UUID, changes []models.PermissionChange) error {
	repository.savedSessionID = sessionID
	repository.savedChanges = append([]models.PermissionChange(nil), changes...)
	return repository.saveChangesErr
}
