// Package adsync snapshots Active Directory users, groups and flattened
// group memberships into the database per "sync run", so later cross-analysis
// against scanned NTFS permissions is a pure SQL join on SIDs.
package adsync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/weibinliao/OpenAD/internal/ad"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const insertBatchSize = 500

// ErrRunNotFound is returned when a sync run id does not exist.
var ErrRunNotFound = errors.New("directory sync run not found")

// Enumerator is the subset of *ad.ADClient the sync needs; a seam for tests.
type Enumerator interface {
	EnumerateSyncUsers(ctx context.Context, pageSize int, fn func(ad.SyncUser) error) error
	EnumerateSyncGroups(ctx context.Context, pageSize int, fn func(ad.SyncGroup) error) error
}

// Service persists directory sync runs and their snapshot records.
type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// HasRunningRun reports whether any sync run is currently in progress.
func (service *Service) HasRunningRun() (bool, error) {
	var count int64
	err := service.db.Model(&models.DirectorySyncRun{}).Where("status = ?", "running").Count(&count).Error
	return count > 0, err
}

// StartRun creates a new run row in the running state and returns it.
func (service *Service) StartRun(connectionID uuid.UUID) (*models.DirectorySyncRun, error) {
	run := &models.DirectorySyncRun{
		ConnectionID: connectionID,
		Status:       "running",
		StartedAt:    time.Now().UTC(),
	}
	if err := service.db.Create(run).Error; err != nil {
		return nil, err
	}
	return run, nil
}

// GetRun returns one run by id.
func (service *Service) GetRun(id uuid.UUID) (*models.DirectorySyncRun, error) {
	var run models.DirectorySyncRun
	err := service.db.First(&run, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// ListRuns returns runs ordered by started_at descending with the total count.
func (service *Service) ListRuns(page, pageSize int) ([]models.DirectorySyncRun, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var total int64
	if err := service.db.Model(&models.DirectorySyncRun{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var runs []models.DirectorySyncRun
	err := service.db.
		Order("started_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&runs).Error
	if err != nil {
		return nil, 0, err
	}

	return runs, total, nil
}

// Run performs the snapshot for an already-created run row: enumerates
// groups, then users, flattens memberships, persists everything in batches
// and finally marks the run completed or failed.
func (service *Service) Run(ctx context.Context, runID uuid.UUID, client Enumerator) error {
	if err := service.run(ctx, runID, client); err != nil {
		service.finishRun(runID, "failed", err.Error(), nil)
		return err
	}
	return nil
}

func (service *Service) run(ctx context.Context, runID uuid.UUID, client Enumerator) error {
	// 1. Groups first: the DN -> group map is needed for flattening.
	groups := make([]ad.SyncGroup, 0, 1024)
	err := client.EnumerateSyncGroups(ctx, insertBatchSize, func(group ad.SyncGroup) error {
		groups = append(groups, group)
		return nil
	})
	if err != nil {
		return fmt.Errorf("enumerate groups: %w", err)
	}

	groupRecords := make([]models.ADGroupRecord, 0, len(groups))
	for _, group := range groups {
		groupRecords = append(groupRecords, models.ADGroupRecord{
			RunID: runID,
			SID:   group.SID,
			Name:  group.Name,
			DN:    group.DN,
			Scope: group.Scope,
		})
	}
	if err := createInBatches(service.db, groupRecords); err != nil {
		return fmt.Errorf("persist groups: %w", err)
	}

	// 2. Users.
	users := make([]ad.SyncUser, 0, 4096)
	err = client.EnumerateSyncUsers(ctx, insertBatchSize, func(user ad.SyncUser) error {
		users = append(users, user)
		return nil
	})
	if err != nil {
		return fmt.Errorf("enumerate users: %w", err)
	}

	userRecords := make([]models.ADUserRecord, 0, len(users))
	for _, user := range users {
		userRecords = append(userRecords, models.ADUserRecord{
			RunID:          runID,
			SID:            user.SID,
			SAMAccountName: user.SAMAccountName,
			UPN:            user.UserPrincipalName,
			DisplayName:    user.DisplayName,
			Email:          user.Email,
			FirstName:      user.FirstName,
			LastName:       user.LastName,
			Department:     user.Department,
			Division:       user.Division,
			Domain:         user.Domain,
			DN:             user.DN,
			Enabled:        user.Enabled,
		})
	}
	if err := createInBatches(service.db, userRecords); err != nil {
		return fmt.Errorf("persist users: %w", err)
	}

	// 3. Flattened memberships (user->group and group->group closures).
	memberships, err := FlattenMemberships(users, groups)
	if err != nil {
		return err
	}
	for index := range memberships {
		memberships[index].RunID = runID
	}
	if err := createInBatches(service.db, memberships); err != nil {
		return fmt.Errorf("persist memberships: %w", err)
	}

	// 4. Finalize.
	counts := map[string]interface{}{
		"user_count":       len(users),
		"group_count":      len(groups),
		"membership_count": len(memberships),
	}

	return service.finishRun(runID, "completed", "", counts)
}

// MarkRunFailed finalizes a run as failed with the given message; used when
// the sync cannot even start (e.g. the LDAP connection fails).
func (service *Service) MarkRunFailed(runID uuid.UUID, message string) error {
	return service.finishRun(runID, "failed", message, nil)
}

func (service *Service) finishRun(runID uuid.UUID, status, errorMessage string, extra map[string]interface{}) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":        status,
		"error_message": errorMessage,
		"finished_at":   &now,
	}
	for key, value := range extra {
		updates[key] = value
	}

	return service.db.Model(&models.DirectorySyncRun{}).Where("id = ?", runID).Updates(updates).Error
}

func createInBatches[T any](db *gorm.DB, records []T) error {
	if len(records) == 0 {
		return nil
	}
	return db.CreateInBatches(records, insertBatchSize).Error
}
