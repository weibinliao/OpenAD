package historyservice

import (
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weibinliao/OpenAD/internal/database"
	"github.com/weibinliao/OpenAD/internal/models"
	"gorm.io/gorm"
)

func TestListSessionsAppliesFiltersAndPagination(t *testing.T) {
	olderCompleted := models.ScanSession{
		ID:               uuid.New(),
		RootPath:         `C:\Engineering`,
		Status:           "completed",
		MaxDepth:         2,
		IncludeInherited: true,
		ItemsScanned:     8,
		PermissionCount:  3,
		StartedAt:        time.Now().Add(-2 * time.Hour).UTC(),
	}
	newerCompleted := models.ScanSession{
		ID:               uuid.New(),
		RootPath:         `C:\Finance`,
		Status:           "completed",
		MaxDepth:         4,
		IncludeInherited: false,
		ItemsScanned:     12,
		PermissionCount:  5,
		StartedAt:        time.Now().Add(-1 * time.Hour).UTC(),
	}
	failedFinance := models.ScanSession{
		ID:               uuid.New(),
		RootPath:         `C:\Finance\Archive`,
		Status:           "failed",
		MaxDepth:         1,
		IncludeInherited: true,
		ItemsScanned:     2,
		PermissionCount:  0,
		StartedAt:        time.Now().UTC(),
	}

	service := NewWithRepository(&stubHistoryRepository{
		sessions: []models.ScanSession{olderCompleted, newerCompleted, failedFinance},
	})

	response, err := service.ListSessions(SessionListFilter{
		Page:     1,
		PageSize: 1,
		Status:   "completed",
		RootPath: "finance",
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Len(t, response.Items, 1)
	assert.Equal(t, newerCompleted.ID, response.Items[0].ID)
	assert.Equal(t, int64(1), response.Pagination.Total)
	assert.Equal(t, 1, response.Pagination.Page)
	assert.Equal(t, 1, response.Pagination.PageSize)
	assert.Equal(t, 1, response.Pagination.TotalPages)
}

func TestGetSessionReturnsValidationAndNotFoundErrors(t *testing.T) {
	t.Run("invalid session id", func(t *testing.T) {
		service := NewWithRepository(&stubHistoryRepository{})

		session, err := service.GetSession("not-a-uuid")

		assert.Nil(t, session)
		assert.ErrorIs(t, err, ErrInvalidSessionID)
	})

	t.Run("session not found", func(t *testing.T) {
		service := NewWithRepository(&stubHistoryRepository{})

		session, err := service.GetSession(uuid.NewString())

		assert.Nil(t, session)
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})

	t.Run("session found", func(t *testing.T) {
		expected := models.ScanSession{
			ID:               uuid.New(),
			RootPath:         `C:\Reports`,
			Status:           "completed",
			MaxDepth:         3,
			IncludeInherited: true,
			ItemsScanned:     6,
			PermissionCount:  2,
			StartedAt:        time.Now().UTC(),
		}
		service := NewWithRepository(&stubHistoryRepository{
			sessions: []models.ScanSession{expected},
		})

		session, err := service.GetSession(expected.ID.String())

		require.NoError(t, err)
		require.NotNil(t, session)
		assert.Equal(t, expected.ID, session.ID)
		assert.Equal(t, expected.RootPath, session.RootPath)
	})
}

func TestGetSessionBundleReturnsSessionAndPermissions(t *testing.T) {
	session := models.ScanSession{
		ID:               uuid.New(),
		RootPath:         `\\server\finance`,
		Status:           "completed",
		MaxDepth:         3,
		IncludeInherited: true,
		ItemsScanned:     5,
		PermissionCount:  2,
		StartedAt:        time.Now().UTC(),
	}
	permissions := []models.Permission{
		{
			ID:                        uuid.New(),
			ScanSessionID:             session.ID,
			Path:                      `\\server\finance`,
			Trustee:                   `EXAMPLE\FinanceTeam`,
			TrusteeSID:                "S-1-5-21-100",
			Rights:                    "Read",
			Type:                      "Allow",
			Inherited:                 false,
			AccountName:               "alice",
			Email:                     "alice@example.com",
			OriginatingGroup:          `EXAMPLE\\FinanceTeam`,
			GroupInheritanceHierarchy: `EXAMPLE\\FinanceTeam`,
			CreatedAt:                 time.Now().UTC(),
		},
	}
	service := NewWithRepository(&stubHistoryRepository{
		sessions: []models.ScanSession{session},
		permissionsBySessionID: map[uuid.UUID][]models.Permission{
			session.ID: permissions,
		},
	})

	response, err := service.GetSessionBundle(session.ID.String())

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, session.ID, response.Session.ID)
	assert.Len(t, response.Permissions, 1)
	assert.Equal(t, permissions[0].Trustee, response.Permissions[0].Trustee)
	assert.Equal(t, permissions[0].OriginatingGroup, response.Permissions[0].OriginatingGroup)
	assert.Equal(t, permissions[0].Email, response.Permissions[0].Email)
}

func TestGetSessionBundleEnrichesLegacyPermissionsWithoutMutatingStoredRows(t *testing.T) {
	db := newHistoryResolverDB(t)
	startedAt := time.Now().UTC()
	run := seedHistoryRun(t, db, startedAt.Add(-time.Hour))
	require.NoError(t, db.Create(&models.ADUserRecord{
		RunID: run.ID, SID: "S-1-5-21-1-2-3-1001", SAMAccountName: "alice", DisplayName: "Alice Adams",
	}).Error)

	session := models.ScanSession{ID: uuid.New(), RootPath: `C:\Share`, Status: "completed", StartedAt: startedAt}
	stored := []models.Permission{{
		ID: uuid.New(), ScanSessionID: session.ID, Path: `C:\Share`, Trustee: "S-1-5-21-1-2-3-1001",
		TrusteeSID: "S-1-5-21-1-2-3-1001", Rights: "Read", Type: "Allow",
	}}
	repository := &stubHistoryRepository{
		sessions:               []models.ScanSession{session},
		permissionsBySessionID: map[uuid.UUID][]models.Permission{session.ID: stored},
	}
	service := NewWithRepositoryAndDatabase(repository, db)

	bundle, err := service.GetSessionBundle(session.ID.String())

	require.NoError(t, err)
	assert.Equal(t, run.ID, bundle.IdentityResolution.DirectorySyncRunID)
	assert.Equal(t, "legacy_inferred_before_scan", bundle.IdentityResolution.Inference)
	require.Len(t, bundle.Permissions, 1)
	assert.Equal(t, "Alice Adams", bundle.Permissions[0].Trustee)
	assert.Equal(t, "snapshot", bundle.Permissions[0].ResolutionSource)
	assert.Equal(t, "S-1-5-21-1-2-3-1001", repository.permissionsBySessionID[session.ID][0].Trustee)
	assert.Empty(t, repository.permissionsBySessionID[session.ID][0].ResolutionSource)
}

func TestSelectLegacyRunPrefersHighestSIDCoverageBeforeScan(t *testing.T) {
	db := newHistoryResolverDB(t)
	scanTime := time.Now().UTC()
	older := seedHistoryRun(t, db, scanTime.Add(-2*time.Hour))
	better := seedHistoryRun(t, db, scanTime.Add(-time.Hour))
	seedHistoryRun(t, db, scanTime.Add(time.Hour))
	require.NoError(t, db.Create(&models.ADUserRecord{RunID: older.ID, SID: "S-1-5-21-a"}).Error)
	require.NoError(t, db.Create(&[]models.ADUserRecord{
		{RunID: better.ID, SID: "S-1-5-21-a"},
		{RunID: better.ID, SID: "S-1-5-21-b"},
	}).Error)

	run, inference, err := selectLegacyRun(db, models.ScanSession{StartedAt: scanTime}, []models.Permission{
		{TrusteeSID: "S-1-5-21-a"},
		{TrusteeSID: "S-1-5-21-b"},
	})

	require.NoError(t, err)
	require.NotNil(t, run)
	assert.Equal(t, better.ID, run.ID)
	assert.Equal(t, "legacy_inferred_before_scan", inference)
}

func TestSelectLegacyRunUsesEarliestSnapshotAfterScan(t *testing.T) {
	db := newHistoryResolverDB(t)
	scanTime := time.Now().UTC()
	earliest := seedHistoryRun(t, db, scanTime.Add(time.Hour))
	seedHistoryRun(t, db, scanTime.Add(2*time.Hour))

	run, inference, err := selectLegacyRun(db, models.ScanSession{StartedAt: scanTime}, nil)

	require.NoError(t, err)
	require.NotNil(t, run)
	assert.Equal(t, earliest.ID, run.ID)
	assert.Equal(t, "legacy_inferred_after_scan", inference)
}

func TestSelectLegacyRunTreatsSnapshotCompletedAfterScanAsLater(t *testing.T) {
	db := newHistoryResolverDB(t)
	scanTime := time.Now().UTC()
	completedBefore := seedHistoryRun(t, db, scanTime.Add(-2*time.Hour))
	finishedAfter := scanTime.Add(30 * time.Minute)
	overlapping := models.DirectorySyncRun{
		ConnectionID: uuid.New(),
		Status:       "completed",
		StartedAt:    scanTime.Add(-time.Hour),
		FinishedAt:   &finishedAfter,
	}
	require.NoError(t, db.Create(&overlapping).Error)
	require.NoError(t, db.Create(&[]models.ADUserRecord{
		{RunID: completedBefore.ID, SID: "S-1-5-21-a"},
		{RunID: overlapping.ID, SID: "S-1-5-21-a"},
	}).Error)

	run, inference, err := selectLegacyRun(db, models.ScanSession{StartedAt: scanTime}, []models.Permission{
		{TrusteeSID: "S-1-5-21-a"},
	})

	require.NoError(t, err)
	require.NotNil(t, run)
	assert.Equal(t, completedBefore.ID, run.ID)
	assert.Equal(t, "legacy_inferred_before_scan", inference)
}

func newHistoryResolverDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.DirectorySyncRun{}, &models.ADUserRecord{}, &models.ADGroupRecord{}, &models.ADMembershipRecord{}))
	return db
}

func seedHistoryRun(t *testing.T, db *gorm.DB, startedAt time.Time) models.DirectorySyncRun {
	t.Helper()
	finishedAt := startedAt.Add(time.Minute)
	run := models.DirectorySyncRun{ConnectionID: uuid.New(), Status: "completed", StartedAt: startedAt, FinishedAt: &finishedAt}
	require.NoError(t, db.Create(&run).Error)
	return run
}

func TestListSessionPermissionsAppliesFiltersAndPagination(t *testing.T) {
	session := models.ScanSession{
		ID:               uuid.New(),
		RootPath:         `C:\Finance`,
		Status:           "completed",
		MaxDepth:         2,
		IncludeInherited: true,
		ItemsScanned:     10,
		PermissionCount:  3,
		StartedAt:        time.Now().UTC(),
	}

	service := NewWithRepository(&stubHistoryRepository{
		sessions: []models.ScanSession{session},
		permissionsBySessionID: map[uuid.UUID][]models.Permission{
			session.ID: {
				{
					ID:            uuid.New(),
					ScanSessionID: session.ID,
					Path:          `C:\Finance`,
					Trustee:       `DOMAIN\Alice`,
					TrusteeSID:    "S-1-5-21-100",
					Rights:        "Read",
					Type:          "Allow",
					Inherited:     false,
					CreatedAt:     time.Now().Add(-1 * time.Minute).UTC(),
				},
				{
					ID:            uuid.New(),
					ScanSessionID: session.ID,
					Path:          `C:\Finance\Payroll`,
					Trustee:       `DOMAIN\Bob`,
					TrusteeSID:    "S-1-5-21-101",
					Rights:        "Write",
					Type:          "Allow",
					Inherited:     true,
					CreatedAt:     time.Now().UTC(),
				},
				{
					ID:            uuid.New(),
					ScanSessionID: session.ID,
					Path:          `C:\Engineering`,
					Trustee:       `DOMAIN\Charlie`,
					TrusteeSID:    "S-1-5-21-102",
					Rights:        "Read",
					Type:          "Allow",
					Inherited:     false,
					CreatedAt:     time.Now().Add(-2 * time.Minute).UTC(),
				},
			},
		},
	})

	inherited := true
	response, err := service.ListSessionPermissions(session.ID.String(), PermissionListFilter{
		Page:      1,
		PageSize:  5,
		Path:      "payroll",
		Trustee:   "bob",
		Inherited: &inherited,
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Len(t, response.Items, 1)
	assert.Equal(t, `DOMAIN\Bob`, response.Items[0].Trustee)
	assert.Equal(t, int64(1), response.Pagination.Total)
	assert.Equal(t, 1, response.Pagination.TotalPages)
}

func TestListSessionChangesAppliesFiltersAndPagination(t *testing.T) {
	session := models.ScanSession{
		ID:               uuid.New(),
		RootPath:         `C:\Finance`,
		Status:           "completed",
		MaxDepth:         2,
		IncludeInherited: true,
		ItemsScanned:     10,
		PermissionCount:  3,
		StartedAt:        time.Now().UTC(),
	}

	service := NewWithRepository(&stubHistoryRepository{
		sessions: []models.ScanSession{session},
		changesBySessionID: map[uuid.UUID][]models.PermissionChange{
			session.ID: {
				{
					ID:            uuid.New(),
					ScanSessionID: session.ID,
					Path:          `C:\Finance\Payroll`,
					Trustee:       `DOMAIN\Alice`,
					TrusteeSID:    "S-1-5-21-100",
					ChangeType:    "added",
					CurrentRights: "Read",
					DetectedAt:    time.Now().UTC(),
				},
				{
					ID:             uuid.New(),
					ScanSessionID:  session.ID,
					Path:           `C:\Engineering`,
					Trustee:        `DOMAIN\Bob`,
					TrusteeSID:     "S-1-5-21-101",
					ChangeType:     "removed",
					PreviousRights: "Write",
					DetectedAt:     time.Now().Add(-1 * time.Minute).UTC(),
				},
			},
		},
	})

	response, err := service.ListSessionChanges(session.ID.String(), ChangeListFilter{
		Page:       1,
		PageSize:   5,
		ChangeType: "added",
		Path:       "payroll",
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Len(t, response.Items, 1)
	assert.Equal(t, "added", response.Items[0].ChangeType)
	assert.Equal(t, int64(1), response.Pagination.Total)
	assert.Equal(t, 1, response.Pagination.TotalPages)

	responseByTrustee, err := service.ListSessionChanges(session.ID.String(), ChangeListFilter{
		Page:     1,
		PageSize: 5,
		Trustee:  "alice",
	})
	require.NoError(t, err)
	require.NotNil(t, responseByTrustee)
	require.Len(t, responseByTrustee.Items, 1)
	assert.Equal(t, `DOMAIN\Alice`, responseByTrustee.Items[0].Trustee)
}

func TestHistoryServiceReturnsRepositoryErrors(t *testing.T) {
	repositoryErr := errors.New("repository unavailable")
	service := NewWithRepository(&stubHistoryRepository{
		listSessionsErr:    repositoryErr,
		getSessionErr:      repositoryErr,
		listPermissionsErr: repositoryErr,
		listChangesErr:     repositoryErr,
	})

	sessions, err := service.ListSessions(SessionListFilter{})
	assert.Nil(t, sessions)
	assert.Equal(t, repositoryErr, err)

	session, err := service.GetSession(uuid.NewString())
	assert.Nil(t, session)
	assert.Equal(t, repositoryErr, err)

	permissions, err := service.ListSessionPermissions(uuid.NewString(), PermissionListFilter{})
	assert.Nil(t, permissions)
	assert.Equal(t, repositoryErr, err)

	changes, err := service.ListSessionChanges(uuid.NewString(), ChangeListFilter{})
	assert.Nil(t, changes)
	assert.Equal(t, repositoryErr, err)
}

func TestHistoryServiceReturnsDatabaseUnavailableWhenUsingDefaultRepository(t *testing.T) {
	previousDB := database.DB
	database.DB = nil
	t.Cleanup(func() {
		database.DB = previousDB
	})

	service := New()

	sessions, err := service.ListSessions(SessionListFilter{})
	assert.Nil(t, sessions)
	assert.ErrorIs(t, err, ErrDatabaseUnavailable)

	session, err := service.GetSession(uuid.NewString())
	assert.Nil(t, session)
	assert.ErrorIs(t, err, ErrDatabaseUnavailable)

	permissions, err := service.ListSessionPermissions(uuid.NewString(), PermissionListFilter{})
	assert.Nil(t, permissions)
	assert.ErrorIs(t, err, ErrDatabaseUnavailable)
}

func TestDatabaseRepositoryReturnsDatabaseUnavailableWhenDatabaseIsNotReady(t *testing.T) {
	previousDB := database.DB
	database.DB = nil
	t.Cleanup(func() {
		database.DB = previousDB
	})

	repository := &databaseRepository{}

	sessions, err := repository.ListSessions()
	assert.Nil(t, sessions)
	assert.ErrorIs(t, err, ErrDatabaseUnavailable)

	session, err := repository.GetSession(uuid.New())
	assert.Nil(t, session)
	assert.ErrorIs(t, err, ErrDatabaseUnavailable)

	permissions, err := repository.ListSessionPermissions(uuid.New())
	assert.Nil(t, permissions)
	assert.ErrorIs(t, err, ErrDatabaseUnavailable)

	changes, err := repository.ListSessionChanges(uuid.New())
	assert.Nil(t, changes)
	assert.ErrorIs(t, err, ErrDatabaseUnavailable)
}

type stubHistoryRepository struct {
	sessions               []models.ScanSession
	permissionsBySessionID map[uuid.UUID][]models.Permission
	changesBySessionID     map[uuid.UUID][]models.PermissionChange
	listSessionsErr        error
	getSessionErr          error
	listPermissionsErr     error
	listChangesErr         error
}

func (repository *stubHistoryRepository) ListSessions() ([]models.ScanSession, error) {
	if repository.listSessionsErr != nil {
		return nil, repository.listSessionsErr
	}

	return append([]models.ScanSession(nil), repository.sessions...), nil
}

func (repository *stubHistoryRepository) GetSession(id uuid.UUID) (*models.ScanSession, error) {
	if repository.getSessionErr != nil {
		return nil, repository.getSessionErr
	}

	for _, session := range repository.sessions {
		if session.ID == id {
			matched := session
			return &matched, nil
		}
	}

	return nil, nil
}

func (repository *stubHistoryRepository) ListSessionPermissions(id uuid.UUID) ([]models.Permission, error) {
	if repository.listPermissionsErr != nil {
		return nil, repository.listPermissionsErr
	}

	permissions := repository.permissionsBySessionID[id]
	return append([]models.Permission(nil), permissions...), nil
}

func (repository *stubHistoryRepository) ListSessionChanges(id uuid.UUID) ([]models.PermissionChange, error) {
	if repository.listChangesErr != nil {
		return nil, repository.listChangesErr
	}

	changes := repository.changesBySessionID[id]
	return append([]models.PermissionChange(nil), changes...), nil
}
