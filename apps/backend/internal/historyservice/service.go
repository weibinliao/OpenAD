package historyservice

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/weibinliao/OpenAD/internal/database"
	"github.com/weibinliao/OpenAD/internal/identityresolution"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/scanner"
	"gorm.io/gorm"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

var (
	ErrDatabaseUnavailable = errors.New("database is not initialized")
	ErrInvalidSessionID    = errors.New("invalid session id")
	ErrSessionNotFound     = errors.New("scan session not found")
)

type repository interface {
	ListSessions() ([]models.ScanSession, error)
	GetSession(id uuid.UUID) (*models.ScanSession, error)
	ListSessionPermissions(id uuid.UUID) ([]models.Permission, error)
	ListSessionChanges(id uuid.UUID) ([]models.PermissionChange, error)
}

type queryRepository interface {
	ListSessionsPage(filter SessionListFilter, page, pageSize int) ([]models.ScanSession, int64, error)
	ListSessionPermissionsPage(id uuid.UUID, filter PermissionListFilter, page, pageSize int) ([]models.Permission, int64, error)
	ListSessionChangesPage(id uuid.UUID, filter ChangeListFilter, page, pageSize int) ([]models.PermissionChange, int64, error)
}

type Service struct {
	repository repository
	db         *gorm.DB
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type SessionListFilter struct {
	Page     int
	PageSize int
	Status   string
	RootPath string
}

type PermissionListFilter struct {
	Page      int
	PageSize  int
	Path      string
	Trustee   string
	Inherited *bool
}

type ChangeListFilter struct {
	Page       int
	PageSize   int
	ChangeType string
	Path       string
	Trustee    string
}

type SessionListResponse struct {
	Items      []models.ScanSession `json:"items"`
	Pagination Pagination           `json:"pagination"`
}

type PermissionListResponse struct {
	Items      []models.Permission `json:"items"`
	Pagination Pagination          `json:"pagination"`
}

type ChangeListResponse struct {
	Items      []models.PermissionChange `json:"items"`
	Pagination Pagination                `json:"pagination"`
}

type SessionBundleResponse struct {
	Session            models.ScanSession          `json:"session"`
	Permissions        []models.Permission         `json:"permissions"`
	IdentityResolution identityresolution.Metadata `json:"identity_resolution"`
}

func New() *Service {
	return NewWithRepositoryAndDatabase(&databaseRepository{}, database.DB)
}

func NewWithRepository(repository repository) *Service {
	return NewWithRepositoryAndDatabase(repository, nil)
}

func NewWithRepositoryAndDatabase(repository repository, db *gorm.DB) *Service {
	if repository == nil {
		repository = &databaseRepository{}
	}

	return &Service{repository: repository, db: db}
}

func (service *Service) ListSessions(filter SessionListFilter) (*SessionListResponse, error) {
	page, pageSize := normalizePagination(filter.Page, filter.PageSize)
	if repository, ok := service.repository.(queryRepository); ok {
		sessions, total, err := repository.ListSessionsPage(filter, page, pageSize)
		if err != nil {
			return nil, err
		}

		return &SessionListResponse{
			Items:      sessions,
			Pagination: buildPagination(page, pageSize, total),
		}, nil
	}

	sessions, err := service.repository.ListSessions()
	if err != nil {
		return nil, err
	}

	filteredSessions := filterSessions(sessions, filter)
	sort.Slice(filteredSessions, func(left, right int) bool {
		return filteredSessions[left].StartedAt.After(filteredSessions[right].StartedAt)
	})

	total := int64(len(filteredSessions))
	filteredSessions = paginateSessions(filteredSessions, page, pageSize)

	return &SessionListResponse{
		Items:      filteredSessions,
		Pagination: buildPagination(page, pageSize, total),
	}, nil
}

func (service *Service) GetSession(id string) (*models.ScanSession, error) {
	sessionID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidSessionID, strings.TrimSpace(id))
	}

	session, err := service.repository.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	if session == nil {
		return nil, ErrSessionNotFound
	}

	return session, nil
}

func (service *Service) GetSessionBundle(id string) (*SessionBundleResponse, error) {
	sessionID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidSessionID, strings.TrimSpace(id))
	}

	session, err := service.repository.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}

	permissions, err := service.repository.ListSessionPermissions(sessionID)
	if err != nil {
		return nil, err
	}
	metadata := identityresolution.Metadata{
		Mode:                     session.IdentityResolutionMode,
		ResolvedPrincipalCount:   session.ResolvedPrincipalCount,
		UnresolvedPrincipalCount: session.UnresolvedPrincipalCount,
		Warning:                  session.IdentityResolutionWarning,
	}
	if session.DirectorySyncRunID != nil {
		metadata.DirectorySyncRunID = *session.DirectorySyncRunID
	}

	if session.IdentityResolutionMode == "" && service.db != nil {
		run, inference, selectErr := selectLegacyRun(service.db, *session, permissions)
		if selectErr != nil {
			return nil, selectErr
		}
		if run != nil {
			resolver := identityresolution.NewService(service.db, identityresolution.Options{RunID: run.ID})
			resolved, resolveErr := resolver.Resolve(context.Background(), modelPermissionsToScanner(permissions))
			if resolveErr != nil {
				return nil, resolveErr
			}
			permissions = scannerPermissionsToModels(permissions, resolved.Permissions)
			metadata = resolved.Metadata
			metadata.Inference = inference
		}
	}
	if metadata.Mode == "" {
		metadata.Mode = "raw"
		metadata.UnresolvedPrincipalCount = countModelPrincipals(permissions)
	}

	return &SessionBundleResponse{
		Session:            *session,
		Permissions:        permissions,
		IdentityResolution: metadata,
	}, nil
}

func (service *Service) ListSessionPermissions(id string, filter PermissionListFilter) (*PermissionListResponse, error) {
	sessionID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidSessionID, strings.TrimSpace(id))
	}

	session, err := service.repository.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}

	page, pageSize := normalizePagination(filter.Page, filter.PageSize)
	if repository, ok := service.repository.(queryRepository); ok {
		permissions, total, err := repository.ListSessionPermissionsPage(sessionID, filter, page, pageSize)
		if err != nil {
			return nil, err
		}

		return &PermissionListResponse{
			Items:      permissions,
			Pagination: buildPagination(page, pageSize, total),
		}, nil
	}

	permissions, err := service.repository.ListSessionPermissions(sessionID)
	if err != nil {
		return nil, err
	}

	filteredPermissions := filterPermissions(permissions, filter)
	sort.Slice(filteredPermissions, func(left, right int) bool {
		if filteredPermissions[left].CreatedAt.Equal(filteredPermissions[right].CreatedAt) {
			if filteredPermissions[left].Path == filteredPermissions[right].Path {
				return filteredPermissions[left].Trustee < filteredPermissions[right].Trustee
			}

			return filteredPermissions[left].Path < filteredPermissions[right].Path
		}

		return filteredPermissions[left].CreatedAt.After(filteredPermissions[right].CreatedAt)
	})

	total := int64(len(filteredPermissions))
	filteredPermissions = paginatePermissions(filteredPermissions, page, pageSize)

	return &PermissionListResponse{
		Items:      filteredPermissions,
		Pagination: buildPagination(page, pageSize, total),
	}, nil
}

func selectLegacyRun(db *gorm.DB, session models.ScanSession, permissions []models.Permission) (*models.DirectorySyncRun, string, error) {
	if db == nil {
		return nil, "", nil
	}
	if session.DirectorySyncRunID != nil {
		var bound models.DirectorySyncRun
		if err := db.Where("id = ? AND status = ?", *session.DirectorySyncRunID, "completed").First(&bound).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, "", nil
			}
			return nil, "", err
		}
		return &bound, "bound", nil
	}

	var runs []models.DirectorySyncRun
	if err := db.Where("status = ?", "completed").Find(&runs).Error; err != nil {
		return nil, "", fmt.Errorf("load directory snapshots: %w", err)
	}
	sort.SliceStable(runs, func(left, right int) bool {
		return directoryRunCompletedAt(runs[left]).Before(directoryRunCompletedAt(runs[right]))
	})
	if len(runs) == 0 {
		return nil, "", nil
	}

	sids := make(map[string]struct{})
	for _, permission := range permissions {
		sid := strings.ToUpper(strings.TrimSpace(permission.TrusteeSID))
		if sid != "" {
			sids[sid] = struct{}{}
		}
	}

	var bestBefore *models.DirectorySyncRun
	bestCoverage := -1
	for index := range runs {
		run := runs[index]
		completedAt := directoryRunCompletedAt(run)
		if completedAt.After(session.StartedAt) {
			continue
		}
		coverage, err := snapshotSIDCoverage(db, run.ID, sids)
		if err != nil {
			return nil, "", err
		}
		if bestBefore == nil || coverage > bestCoverage || (coverage == bestCoverage && completedAt.After(directoryRunCompletedAt(*bestBefore))) {
			candidate := run
			bestBefore = &candidate
			bestCoverage = coverage
		}
	}
	if bestBefore != nil {
		return bestBefore, "legacy_inferred_before_scan", nil
	}

	for index := range runs {
		if directoryRunCompletedAt(runs[index]).After(session.StartedAt) {
			candidate := runs[index]
			return &candidate, "legacy_inferred_after_scan", nil
		}
	}
	return nil, "", nil
}

func directoryRunCompletedAt(run models.DirectorySyncRun) time.Time {
	if run.FinishedAt != nil && !run.FinishedAt.IsZero() {
		return run.FinishedAt.UTC()
	}
	return run.StartedAt.UTC()
}

func snapshotSIDCoverage(db *gorm.DB, runID uuid.UUID, wanted map[string]struct{}) (int, error) {
	if len(wanted) == 0 {
		return 0, nil
	}
	var users []models.ADUserRecord
	if err := db.Select("sid").Where("run_id = ?", runID).Find(&users).Error; err != nil {
		return 0, fmt.Errorf("load snapshot user SIDs: %w", err)
	}
	var groups []models.ADGroupRecord
	if err := db.Select("sid").Where("run_id = ?", runID).Find(&groups).Error; err != nil {
		return 0, fmt.Errorf("load snapshot group SIDs: %w", err)
	}
	found := make(map[string]struct{})
	for _, user := range users {
		key := strings.ToUpper(strings.TrimSpace(user.SID))
		if _, ok := wanted[key]; ok {
			found[key] = struct{}{}
		}
	}
	for _, group := range groups {
		key := strings.ToUpper(strings.TrimSpace(group.SID))
		if _, ok := wanted[key]; ok {
			found[key] = struct{}{}
		}
	}
	return len(found), nil
}

func modelPermissionsToScanner(permissions []models.Permission) []scanner.Permission {
	result := make([]scanner.Permission, 0, len(permissions))
	for _, permission := range permissions {
		result = append(result, scanner.Permission{
			Path: permission.Path, Trustee: permission.Trustee, TrusteeSID: permission.TrusteeSID,
			Rights: permission.Rights, Type: permission.Type, Inherited: permission.Inherited,
			Source: permission.Source, AppliesTo: permission.AppliesTo, AccountType: permission.AccountType,
			AccessMask: permission.AccessMask, RiskLevel: permission.RiskLevel, ParentDelta: permission.ParentDelta,
			AccountName: permission.AccountName, FirstName: permission.FirstName, LastName: permission.LastName,
			Email: permission.Email, Department: permission.Department, Division: permission.Division, Domain: permission.Domain,
			OriginatingGroup: permission.OriginatingGroup, GroupInheritanceHierarchy: permission.GroupInheritanceHierarchy,
			ResolutionSource: permission.ResolutionSource, ResolutionReason: permission.ResolutionReason,
		})
	}
	return result
}

func scannerPermissionsToModels(original []models.Permission, resolved []scanner.Permission) []models.Permission {
	result := make([]models.Permission, 0, len(resolved))
	for _, permission := range resolved {
		template := models.Permission{}
		for _, candidate := range original {
			if candidate.Path == permission.Path && candidate.Rights == permission.Rights && candidate.Type == permission.Type && candidate.Inherited == permission.Inherited {
				template = candidate
				break
			}
		}
		template.Path = permission.Path
		template.Trustee = permission.Trustee
		template.TrusteeSID = permission.TrusteeSID
		template.Rights = permission.Rights
		template.Type = permission.Type
		template.Inherited = permission.Inherited
		template.Source = permission.Source
		template.AppliesTo = permission.AppliesTo
		template.AccountType = permission.AccountType
		template.AccessMask = permission.AccessMask
		template.RiskLevel = permission.RiskLevel
		template.ParentDelta = permission.ParentDelta
		template.AccountName = permission.AccountName
		template.FirstName = permission.FirstName
		template.LastName = permission.LastName
		template.Email = permission.Email
		template.Department = permission.Department
		template.Division = permission.Division
		template.Domain = permission.Domain
		template.OriginatingGroup = permission.OriginatingGroup
		template.GroupInheritanceHierarchy = permission.GroupInheritanceHierarchy
		template.ResolutionSource = permission.ResolutionSource
		template.ResolutionReason = permission.ResolutionReason
		result = append(result, template)
	}
	return result
}

func countModelPrincipals(permissions []models.Permission) int {
	values := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		value := strings.TrimSpace(permission.TrusteeSID)
		if value == "" {
			value = strings.TrimSpace(permission.Trustee)
		}
		if value != "" {
			values[strings.ToUpper(value)] = struct{}{}
		}
	}
	return len(values)
}

func (service *Service) ListSessionChanges(id string, filter ChangeListFilter) (*ChangeListResponse, error) {
	sessionID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidSessionID, strings.TrimSpace(id))
	}

	session, err := service.repository.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}

	page, pageSize := normalizePagination(filter.Page, filter.PageSize)
	if repository, ok := service.repository.(queryRepository); ok {
		changes, total, err := repository.ListSessionChangesPage(sessionID, filter, page, pageSize)
		if err != nil {
			return nil, err
		}

		return &ChangeListResponse{
			Items:      changes,
			Pagination: buildPagination(page, pageSize, total),
		}, nil
	}

	changes, err := service.repository.ListSessionChanges(sessionID)
	if err != nil {
		return nil, err
	}

	filteredChanges := filterChanges(changes, filter)
	sort.Slice(filteredChanges, func(left, right int) bool {
		if filteredChanges[left].DetectedAt.Equal(filteredChanges[right].DetectedAt) {
			if filteredChanges[left].Path == filteredChanges[right].Path {
				return filteredChanges[left].TrusteeSID < filteredChanges[right].TrusteeSID
			}

			return filteredChanges[left].Path < filteredChanges[right].Path
		}

		return filteredChanges[left].DetectedAt.After(filteredChanges[right].DetectedAt)
	})

	total := int64(len(filteredChanges))
	filteredChanges = paginateChanges(filteredChanges, page, pageSize)

	return &ChangeListResponse{
		Items:      filteredChanges,
		Pagination: buildPagination(page, pageSize, total),
	}, nil
}

func normalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = defaultPageSize
	}

	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	return page, pageSize
}

func buildPagination(page, pageSize int, total int64) Pagination {
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return Pagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}

func filterSessions(sessions []models.ScanSession, filter SessionListFilter) []models.ScanSession {
	status := strings.TrimSpace(filter.Status)
	rootPath := strings.ToLower(strings.TrimSpace(filter.RootPath))
	filteredSessions := make([]models.ScanSession, 0, len(sessions))

	for _, session := range sessions {
		if status != "" && !strings.EqualFold(session.Status, status) {
			continue
		}

		if rootPath != "" && !strings.Contains(strings.ToLower(session.RootPath), rootPath) {
			continue
		}

		filteredSessions = append(filteredSessions, session)
	}

	return filteredSessions
}

func filterPermissions(permissions []models.Permission, filter PermissionListFilter) []models.Permission {
	path := strings.ToLower(strings.TrimSpace(filter.Path))
	trustee := strings.ToLower(strings.TrimSpace(filter.Trustee))
	filteredPermissions := make([]models.Permission, 0, len(permissions))

	for _, permission := range permissions {
		if path != "" && !strings.Contains(strings.ToLower(permission.Path), path) {
			continue
		}

		if trustee != "" && !strings.Contains(strings.ToLower(permission.Trustee), trustee) {
			continue
		}

		if filter.Inherited != nil && permission.Inherited != *filter.Inherited {
			continue
		}

		filteredPermissions = append(filteredPermissions, permission)
	}

	return filteredPermissions
}

func filterChanges(changes []models.PermissionChange, filter ChangeListFilter) []models.PermissionChange {
	changeType := strings.ToLower(strings.TrimSpace(filter.ChangeType))
	path := strings.ToLower(strings.TrimSpace(filter.Path))
	trustee := strings.ToLower(strings.TrimSpace(filter.Trustee))
	filteredChanges := make([]models.PermissionChange, 0, len(changes))

	for _, change := range changes {
		if changeType != "" && !strings.EqualFold(change.ChangeType, changeType) {
			continue
		}

		if path != "" && !strings.Contains(strings.ToLower(change.Path), path) {
			continue
		}

		if trustee != "" &&
			!strings.Contains(strings.ToLower(change.TrusteeSID), trustee) &&
			!strings.Contains(strings.ToLower(change.Trustee), trustee) {
			continue
		}

		filteredChanges = append(filteredChanges, change)
	}

	return filteredChanges
}

func paginateSessions(sessions []models.ScanSession, page, pageSize int) []models.ScanSession {
	startIndex := (page - 1) * pageSize
	if startIndex >= len(sessions) {
		return []models.ScanSession{}
	}

	endIndex := startIndex + pageSize
	if endIndex > len(sessions) {
		endIndex = len(sessions)
	}

	return sessions[startIndex:endIndex]
}

func paginatePermissions(permissions []models.Permission, page, pageSize int) []models.Permission {
	startIndex := (page - 1) * pageSize
	if startIndex >= len(permissions) {
		return []models.Permission{}
	}

	endIndex := startIndex + pageSize
	if endIndex > len(permissions) {
		endIndex = len(permissions)
	}

	return permissions[startIndex:endIndex]
}

func paginateChanges(changes []models.PermissionChange, page, pageSize int) []models.PermissionChange {
	startIndex := (page - 1) * pageSize
	if startIndex >= len(changes) {
		return []models.PermissionChange{}
	}

	endIndex := startIndex + pageSize
	if endIndex > len(changes) {
		endIndex = len(changes)
	}

	return changes[startIndex:endIndex]
}

type databaseRepository struct{}

func (repository *databaseRepository) ListSessions() ([]models.ScanSession, error) {
	if !database.Ready() {
		return nil, ErrDatabaseUnavailable
	}

	sessions := make([]models.ScanSession, 0)
	if err := database.DB.Find(&sessions).Error; err != nil {
		return nil, err
	}

	return sessions, nil
}

func (repository *databaseRepository) ListSessionsPage(filter SessionListFilter, page, pageSize int) ([]models.ScanSession, int64, error) {
	if !database.Ready() {
		return nil, 0, ErrDatabaseUnavailable
	}

	query := database.DB.Model(&models.ScanSession{})
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if rootPath := strings.TrimSpace(filter.RootPath); rootPath != "" {
		query = query.Where("LOWER(root_path) LIKE ?", "%"+strings.ToLower(rootPath)+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sessions := make([]models.ScanSession, 0, pageSize)
	if total == 0 {
		return sessions, 0, nil
	}

	offset := (page - 1) * pageSize
	if err := query.Order("started_at DESC").Limit(pageSize).Offset(offset).Find(&sessions).Error; err != nil {
		return nil, 0, err
	}

	return sessions, total, nil
}

func (repository *databaseRepository) GetSession(id uuid.UUID) (*models.ScanSession, error) {
	if !database.Ready() {
		return nil, ErrDatabaseUnavailable
	}

	var session models.ScanSession
	if err := database.DB.First(&session, "id = ?", id).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		return nil, nil
	}

	return &session, nil
}

func (repository *databaseRepository) ListSessionPermissions(id uuid.UUID) ([]models.Permission, error) {
	if !database.Ready() {
		return nil, ErrDatabaseUnavailable
	}

	permissions := make([]models.Permission, 0)
	if err := database.DB.Where("scan_session_id = ?", id).Find(&permissions).Error; err != nil {
		return nil, err
	}

	return permissions, nil
}

func (repository *databaseRepository) ListSessionPermissionsPage(id uuid.UUID, filter PermissionListFilter, page, pageSize int) ([]models.Permission, int64, error) {
	if !database.Ready() {
		return nil, 0, ErrDatabaseUnavailable
	}

	query := database.DB.Model(&models.Permission{}).Where("scan_session_id = ?", id)
	if path := strings.TrimSpace(filter.Path); path != "" {
		query = query.Where("LOWER(path) LIKE ?", "%"+strings.ToLower(path)+"%")
	}
	if trustee := strings.TrimSpace(filter.Trustee); trustee != "" {
		term := "%" + strings.ToLower(trustee) + "%"
		query = query.Where("LOWER(trustee) LIKE ? OR LOWER(trustee_s_id) LIKE ? OR LOWER(source) LIKE ?", term, term, term)
	}
	if filter.Inherited != nil {
		query = query.Where("inherited = ?", *filter.Inherited)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	permissions := make([]models.Permission, 0, pageSize)
	if total == 0 {
		return permissions, 0, nil
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Order("path ASC").Order("trustee ASC").Limit(pageSize).Offset(offset).Find(&permissions).Error; err != nil {
		return nil, 0, err
	}

	return permissions, total, nil
}

func (repository *databaseRepository) ListSessionChanges(id uuid.UUID) ([]models.PermissionChange, error) {
	if !database.Ready() {
		return nil, ErrDatabaseUnavailable
	}

	changes := make([]models.PermissionChange, 0)
	if err := database.DB.Where("scan_session_id = ?", id).Find(&changes).Error; err != nil {
		return nil, err
	}

	return changes, nil
}

func (repository *databaseRepository) ListSessionChangesPage(id uuid.UUID, filter ChangeListFilter, page, pageSize int) ([]models.PermissionChange, int64, error) {
	if !database.Ready() {
		return nil, 0, ErrDatabaseUnavailable
	}

	query := database.DB.Model(&models.PermissionChange{}).Where("scan_session_id = ?", id)
	if changeType := strings.TrimSpace(filter.ChangeType); changeType != "" {
		query = query.Where("LOWER(change_type) = ?", strings.ToLower(changeType))
	}
	if path := strings.TrimSpace(filter.Path); path != "" {
		query = query.Where("LOWER(path) LIKE ?", "%"+strings.ToLower(path)+"%")
	}
	if trustee := strings.TrimSpace(filter.Trustee); trustee != "" {
		term := "%" + strings.ToLower(trustee) + "%"
		query = query.Where("LOWER(trustee) LIKE ? OR LOWER(trustee_s_id) LIKE ?", term, term)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	changes := make([]models.PermissionChange, 0, pageSize)
	if total == 0 {
		return changes, 0, nil
	}

	offset := (page - 1) * pageSize
	if err := query.Order("detected_at DESC").Order("path ASC").Order("trustee_s_id ASC").Limit(pageSize).Offset(offset).Find(&changes).Error; err != nil {
		return nil, 0, err
	}

	return changes, total, nil
}
