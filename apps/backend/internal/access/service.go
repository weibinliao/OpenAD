// Package access is the cross-analysis "access engine": it joins the
// persisted Active Directory snapshot (directory sync runs, ad_users,
// ad_groups, ad_memberships) with persisted NTFS scan permissions so the
// product can answer two questions:
//
//  1. By user: "What can this principal access, and why?"
//  2. By resource: "Who can access this path, and why?"
//
// Everything works on already-persisted data; no live LDAP or filesystem
// access happens here.
package access

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/weibinliao/OpenAD/internal/models"
	"gorm.io/gorm"
)

// Sentinel errors so handlers can map failures to precise HTTP statuses.
var (
	// ErrPrincipalRequired is returned when ByUser is called without a principal.
	ErrPrincipalRequired = errors.New("principal is required")
	// ErrPathRequired is returned when ByResource is called without a path prefix.
	ErrPathRequired = errors.New("path_prefix is required")
	// ErrNoCompletedSyncRun means no completed directory sync snapshot exists yet.
	ErrNoCompletedSyncRun = errors.New("no completed directory sync run exists; run a directory sync first")
	// ErrSyncRunNotFound means the explicitly requested sync run id does not exist.
	ErrSyncRunNotFound = errors.New("directory sync run not found")
	// ErrNoScanSessions means no completed scan sessions exist yet.
	ErrNoScanSessions = errors.New("no completed scan sessions exist; run a folder scan first")
	// ErrSessionNotFound means an explicitly requested scan session id does not exist.
	ErrSessionNotFound = errors.New("scan session not found")
	// ErrNoMatchingSession means no completed scan session covers the requested path.
	ErrNoMatchingSession = errors.New("no completed scan session covers the requested path; scan the folder first")
	// ErrPrincipalNotFound means the principal is not part of the sync snapshot.
	ErrPrincipalNotFound = errors.New("principal not found in the directory snapshot")
)

// trusteeSIDFallbackColumn is the column GORM's default naming strategy
// generates for the Permission.TrusteeSID field ("SID" is not one of GORM's
// common initialisms, so it splits into "s_id"). Verified empirically against
// schema.Parse in TestTrusteeSIDColumnName.
const trusteeSIDFallbackColumn = "trustee_s_id"

// wellKnownSIDs maps well-known Windows SIDs to friendly display names.
// These trustees can never resolve against the AD snapshot but must still be
// reported (an ACE for Everyone is usually the most important row).
var wellKnownSIDs = map[string]string{
	"S-1-0-0":      "Nobody",
	"S-1-1-0":      "Everyone",
	"S-1-3-0":      "CREATOR OWNER",
	"S-1-3-1":      "CREATOR GROUP",
	"S-1-5-4":      "NT AUTHORITY\\INTERACTIVE",
	"S-1-5-7":      "NT AUTHORITY\\ANONYMOUS LOGON",
	"S-1-5-9":      "NT AUTHORITY\\ENTERPRISE DOMAIN CONTROLLERS",
	"S-1-5-11":     "NT AUTHORITY\\Authenticated Users",
	"S-1-5-18":     "NT AUTHORITY\\SYSTEM",
	"S-1-5-19":     "NT AUTHORITY\\LOCAL SERVICE",
	"S-1-5-20":     "NT AUTHORITY\\NETWORK SERVICE",
	"S-1-5-32-544": "BUILTIN\\Administrators",
	"S-1-5-32-545": "BUILTIN\\Users",
	"S-1-5-32-546": "BUILTIN\\Guests",
	"S-1-5-32-547": "BUILTIN\\Power Users",
	"S-1-5-32-551": "BUILTIN\\Backup Operators",
}

// Service answers by-user and by-resource access questions from the database.
type Service struct {
	db               *gorm.DB
	trusteeSIDColumn string
}

// NewService builds a Service on top of an initialized *gorm.DB. The trustee
// SID column name is resolved from the parsed model schema so the query never
// drifts from what AutoMigrate actually created.
func NewService(db *gorm.DB) *Service {
	column := trusteeSIDFallbackColumn
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(&models.Permission{}); err == nil {
		if field := statement.Schema.LookUpField("TrusteeSID"); field != nil && field.DBName != "" {
			column = field.DBName
		}
	}

	return &Service{db: db, trusteeSIDColumn: column}
}

// --- Shared result types -----------------------------------------------

// Why explains how one permission row applies to the queried user.
type Why struct {
	Kind        string `json:"kind"` // direct | group
	Description string `json:"description"`
	GroupName   string `json:"group_name,omitempty"`
	GroupSID    string `json:"group_sid,omitempty"`
	ViaChain    string `json:"via_chain,omitempty"`
}

// AccessEntry is one effective permission row annotated with its why-chain.
type AccessEntry struct {
	SessionID  uuid.UUID `json:"session_id"`
	RootPath   string    `json:"root_path"`
	Path       string    `json:"path"`
	Rights     string    `json:"rights"`
	Type       string    `json:"type"` // allow | deny
	Inherited  bool      `json:"inherited"`
	RiskLevel  string    `json:"risk_level,omitempty"`
	TrusteeSID string    `json:"trustee_sid"`
	Trustee    string    `json:"trustee,omitempty"`
	Why        Why       `json:"why"`
}

// SessionInfo describes one scan session consulted for the answer.
type SessionInfo struct {
	ID       uuid.UUID `json:"id"`
	RootPath string    `json:"root_path"`
	Status   string    `json:"status"`
}

// --- ByUser --------------------------------------------------------------

// ByUserInput selects the principal and, optionally, which snapshot and scan
// sessions to consult.
type ByUserInput struct {
	// Principal is a SID, sAMAccountName or UPN.
	Principal string
	// SyncRunID pins the AD snapshot; zero value means latest completed run.
	SyncRunID uuid.UUID
	// SessionIDs pin the scan sessions; empty means the latest completed
	// session per distinct root path.
	SessionIDs []uuid.UUID
}

// UserInfo is the resolved snapshot user.
type UserInfo struct {
	SID            string `json:"sid"`
	SAMAccountName string `json:"sam_account_name"`
	UPN            string `json:"upn"`
	DisplayName    string `json:"display_name"`
	Email          string `json:"email"`
	Department     string `json:"department"`
	Enabled        bool   `json:"enabled"`
}

// GroupMembership is one flattened group edge for the resolved user.
type GroupMembership struct {
	GroupSID  string `json:"group_sid"`
	GroupName string `json:"group_name"`
	Direct    bool   `json:"direct"`
	ViaChain  string `json:"via_chain,omitempty"`
}

// RootPathEntries groups access entries by the scan session root path.
type RootPathEntries struct {
	RootPath  string        `json:"root_path"`
	SessionID uuid.UUID     `json:"session_id"`
	Entries   []AccessEntry `json:"entries"`
}

// ByUserCounts summarizes the by-user answer.
type ByUserCounts struct {
	Total    int `json:"total"`
	Direct   int `json:"direct"`
	ViaGroup int `json:"via_group"`
	Allow    int `json:"allow"`
	Deny     int `json:"deny"`
}

// ByUserResult is the full "what can this principal access" answer.
type ByUserResult struct {
	SyncRunID  uuid.UUID         `json:"sync_run_id"`
	User       UserInfo          `json:"user"`
	GroupCount int               `json:"group_count"`
	Groups     []GroupMembership `json:"groups"`
	Sessions   []SessionInfo     `json:"sessions"`
	Entries    []AccessEntry     `json:"entries"`
	ByRootPath []RootPathEntries `json:"by_root_path"`
	Counts     ByUserCounts      `json:"counts"`
}

// ByUser answers "what can this principal access, and why?".
func (service *Service) ByUser(input ByUserInput) (*ByUserResult, error) {
	principal := strings.TrimSpace(input.Principal)
	if principal == "" {
		return nil, ErrPrincipalRequired
	}

	run, err := service.resolveSyncRun(input.SyncRunID)
	if err != nil {
		return nil, err
	}

	user, err := service.resolveUser(run.ID, principal)
	if err != nil {
		return nil, err
	}

	// Flattened memberships already include transitive edges, so a single
	// member_sid lookup yields every group whose ACEs apply to this user.
	var memberships []models.ADMembershipRecord
	err = service.db.
		Where("run_id = ? AND member_sid = ?", run.ID, user.SID).
		Find(&memberships).Error
	if err != nil {
		return nil, fmt.Errorf("load memberships: %w", err)
	}

	groupSIDs := make([]string, 0, len(memberships))
	for _, membership := range memberships {
		groupSIDs = append(groupSIDs, membership.GroupSID)
	}
	groupNames, err := service.groupNames(run.ID, groupSIDs)
	if err != nil {
		return nil, err
	}

	membershipBySID := make(map[string]models.ADMembershipRecord, len(memberships))
	groups := make([]GroupMembership, 0, len(memberships))
	for _, membership := range memberships {
		membershipBySID[membership.GroupSID] = membership
		groups = append(groups, GroupMembership{
			GroupSID:  membership.GroupSID,
			GroupName: groupNames[membership.GroupSID],
			Direct:    membership.Direct,
			ViaChain:  membership.ViaChain,
		})
	}
	sort.Slice(groups, func(left, right int) bool { return groups[left].GroupSID < groups[right].GroupSID })

	sessions, err := service.resolveSessions(input.SessionIDs)
	if err != nil {
		return nil, err
	}
	sessionByID := make(map[uuid.UUID]models.ScanSession, len(sessions))
	sessionIDs := make([]uuid.UUID, 0, len(sessions))
	sessionInfos := make([]SessionInfo, 0, len(sessions))
	for _, session := range sessions {
		sessionByID[session.ID] = session
		sessionIDs = append(sessionIDs, session.ID)
		sessionInfos = append(sessionInfos, SessionInfo{ID: session.ID, RootPath: session.RootPath, Status: session.Status})
	}

	trusteeSIDs := append([]string{user.SID}, groupSIDs...)
	var permissions []models.Permission
	err = service.db.
		Where("scan_session_id IN ?", sessionIDs).
		Where(fmt.Sprintf("%s IN ?", service.trusteeSIDColumn), trusteeSIDs).
		Order("path ASC").
		Find(&permissions).Error
	if err != nil {
		return nil, fmt.Errorf("load permissions: %w", err)
	}

	result := &ByUserResult{
		SyncRunID: run.ID,
		User: UserInfo{
			SID:            user.SID,
			SAMAccountName: user.SAMAccountName,
			UPN:            user.UPN,
			DisplayName:    user.DisplayName,
			Email:          user.Email,
			Department:     user.Department,
			Enabled:        user.Enabled,
		},
		GroupCount: len(groups),
		Groups:     groups,
		Sessions:   sessionInfos,
		Entries:    make([]AccessEntry, 0, len(permissions)),
		ByRootPath: make([]RootPathEntries, 0),
	}

	groupedIndex := make(map[uuid.UUID]int)
	for _, permission := range permissions {
		session := sessionByID[permission.ScanSessionID]

		why := Why{Kind: "direct", Description: "direct"}
		if permission.TrusteeSID != user.SID {
			membership := membershipBySID[permission.TrusteeSID]
			groupName := groupNames[permission.TrusteeSID]
			if groupName == "" {
				groupName = permission.TrusteeSID
			}
			why = Why{
				Kind:        "group",
				Description: fmt.Sprintf("via group %s", groupName),
				GroupName:   groupName,
				GroupSID:    permission.TrusteeSID,
				ViaChain:    membership.ViaChain,
			}
			if membership.ViaChain != "" {
				why.Description = fmt.Sprintf("via group %s (%s)", groupName, membership.ViaChain)
			}
		}

		entry := AccessEntry{
			SessionID:  permission.ScanSessionID,
			RootPath:   session.RootPath,
			Path:       permission.Path,
			Rights:     permission.Rights,
			Type:       permission.Type,
			Inherited:  permission.Inherited,
			RiskLevel:  permission.RiskLevel,
			TrusteeSID: permission.TrusteeSID,
			Trustee:    permission.Trustee,
			Why:        why,
		}
		result.Entries = append(result.Entries, entry)

		index, found := groupedIndex[permission.ScanSessionID]
		if !found {
			index = len(result.ByRootPath)
			groupedIndex[permission.ScanSessionID] = index
			result.ByRootPath = append(result.ByRootPath, RootPathEntries{
				RootPath:  session.RootPath,
				SessionID: session.ID,
				Entries:   make([]AccessEntry, 0, 8),
			})
		}
		result.ByRootPath[index].Entries = append(result.ByRootPath[index].Entries, entry)

		result.Counts.Total++
		if why.Kind == "direct" {
			result.Counts.Direct++
		} else {
			result.Counts.ViaGroup++
		}
		if strings.EqualFold(permission.Type, "deny") {
			result.Counts.Deny++
		} else {
			result.Counts.Allow++
		}
	}

	return result, nil
}

// --- ByResource -----------------------------------------------------------

// ByResourceInput selects the path and, optionally, which session/snapshot to
// consult.
type ByResourceInput struct {
	// PathPrefix is the resource path; matching is exact or prefix.
	PathPrefix string
	// SessionID pins the scan session; zero value means the latest completed
	// session whose root path covers (or is contained in) PathPrefix.
	SessionID uuid.UUID
	// SyncRunID pins the AD snapshot; zero value means latest completed run.
	SyncRunID uuid.UUID
}

// ResourcePrincipal is one principal that can touch the resource.
type ResourcePrincipal struct {
	SID         string   `json:"sid"`
	Name        string   `json:"name"`
	Source      string   `json:"source"` // group | user | group-member | unresolved
	Rights      []string `json:"rights"`
	Types       []string `json:"types"` // allow / deny seen for this principal
	RiskLevel   string   `json:"risk_level,omitempty"`
	GroupName   string   `json:"group_name,omitempty"`
	GroupSID    string   `json:"group_sid,omitempty"`
	ViaChain    string   `json:"via_chain,omitempty"`
	Paths       []string `json:"paths"`
	Enabled     *bool    `json:"enabled,omitempty"`
	MemberCount int      `json:"member_count,omitempty"`
}

// ResourceACE is one raw permission row under the path prefix.
type ResourceACE struct {
	Path       string `json:"path"`
	Trustee    string `json:"trustee,omitempty"`
	TrusteeSID string `json:"trustee_sid"`
	Rights     string `json:"rights"`
	Type       string `json:"type"`
	Inherited  bool   `json:"inherited"`
	RiskLevel  string `json:"risk_level,omitempty"`
}

// ByResourceResult is the full "who can access this path" answer.
type ByResourceResult struct {
	PathPrefix string              `json:"path_prefix"`
	Session    SessionInfo         `json:"session"`
	SyncRunID  uuid.UUID           `json:"sync_run_id"`
	Principals []ResourcePrincipal `json:"principals"`
	ACEs       []ResourceACE       `json:"aces"`
	Counts     ByResourceCounts    `json:"counts"`
}

// ByResourceCounts summarizes the by-resource answer.
type ByResourceCounts struct {
	ACEs       int `json:"aces"`
	Principals int `json:"principals"`
	Groups     int `json:"groups"`
	Users      int `json:"users"`
	ViaGroups  int `json:"via_groups"`
	Unresolved int `json:"unresolved"`
}

// ByResource answers "who can access this path, and why?".
func (service *Service) ByResource(input ByResourceInput) (*ByResourceResult, error) {
	pathPrefix := strings.TrimSpace(input.PathPrefix)
	if pathPrefix == "" {
		return nil, ErrPathRequired
	}

	run, err := service.resolveSyncRun(input.SyncRunID)
	if err != nil {
		return nil, err
	}

	session, err := service.resolveSessionForPath(input.SessionID, pathPrefix)
	if err != nil {
		return nil, err
	}

	// Exact match or everything below the prefix. Escape LIKE wildcards in
	// the path so folders containing % or _ do not over-match.
	likePrefix := escapeLike(strings.ToLower(pathPrefix)) + "%"
	var permissions []models.Permission
	err = service.db.
		Where("scan_session_id = ?", session.ID).
		Where(`LOWER(path) = ? OR LOWER(path) LIKE ? ESCAPE '\'`, strings.ToLower(pathPrefix), likePrefix).
		Order("path ASC").
		Find(&permissions).Error
	if err != nil {
		return nil, fmt.Errorf("load permissions: %w", err)
	}

	// Classify every distinct trustee against the snapshot.
	trusteeSIDs := make([]string, 0, len(permissions))
	seenTrustees := make(map[string]struct{}, len(permissions))
	sourceGroupNames := make([]string, 0)
	seenSourceGroups := make(map[string]struct{})
	for _, permission := range permissions {
		if _, seen := seenTrustees[permission.TrusteeSID]; seen {
			// The same trustee can appear through multiple source groups, so
			// group-name collection must continue below.
		} else {
			seenTrustees[permission.TrusteeSID] = struct{}{}
			trusteeSIDs = append(trusteeSIDs, permission.TrusteeSID)
		}
		if groupName := normalizeResourceGroupName(permission.OriginatingGroup); groupName != "" {
			if _, seen := seenSourceGroups[groupName]; !seen {
				seenSourceGroups[groupName] = struct{}{}
				sourceGroupNames = append(sourceGroupNames, groupName)
			}
		}
	}

	usersBySID, err := service.usersBySID(run.ID, trusteeSIDs)
	if err != nil {
		return nil, err
	}
	groupsBySID, groupsByName, err := service.resourceGroups(run.ID, trusteeSIDs, sourceGroupNames)
	if err != nil {
		return nil, err
	}

	result := &ByResourceResult{
		PathPrefix: pathPrefix,
		Session:    SessionInfo{ID: session.ID, RootPath: session.RootPath, Status: session.Status},
		SyncRunID:  run.ID,
		Principals: make([]ResourcePrincipal, 0),
		ACEs:       make([]ResourceACE, 0, len(permissions)),
	}

	// Aggregate principals keyed by (sid, source, via-group) so one user
	// reached both directly and through a group keeps both explanations.
	principalIndex := make(map[string]int)
	groupMembers := make(map[string]map[string]struct{})
	groupKeys := make(map[string]string)
	addPrincipal := func(candidate ResourcePrincipal, right, aceType, risk, path string) {
		key := resourcePrincipalKey(candidate)
		index, found := principalIndex[key]
		if !found {
			index = len(result.Principals)
			principalIndex[key] = index
			result.Principals = append(result.Principals, candidate)
		}
		principal := &result.Principals[index]
		principal.Rights = appendUnique(principal.Rights, right)
		principal.Types = appendUnique(principal.Types, strings.ToLower(aceType))
		principal.Paths = appendUnique(principal.Paths, path)
		principal.RiskLevel = highestRisk(principal.RiskLevel, risk)
	}
	addGroup := func(group models.ADGroupRecord, fallbackName string, permission models.Permission) string {
		name := strings.TrimSpace(group.Name)
		if name == "" {
			name = strings.TrimSpace(fallbackName)
		}
		groupKey := resourceGroupKey(group.SID, name)
		addPrincipal(ResourcePrincipal{
			SID:       strings.TrimSpace(group.SID),
			Name:      name,
			Source:    "group",
			GroupName: name,
			GroupSID:  strings.TrimSpace(group.SID),
		}, permission.Rights, permission.Type, permission.RiskLevel, permission.Path)
		groupKeys[groupKey] = resourcePrincipalKey(ResourcePrincipal{
			SID:       strings.TrimSpace(group.SID),
			Name:      name,
			Source:    "group",
			GroupName: name,
			GroupSID:  strings.TrimSpace(group.SID),
		})
		if _, found := groupMembers[groupKey]; !found {
			groupMembers[groupKey] = make(map[string]struct{})
		}
		return groupKey
	}
	addGroupMember := func(group models.ADGroupRecord, fallbackName string, permission models.Permission, user models.ADUserRecord, viaChain string) {
		groupKey := addGroup(group, fallbackName, permission)
		enabled := user.Enabled
		groupName := strings.TrimSpace(group.Name)
		if groupName == "" {
			groupName = strings.TrimSpace(fallbackName)
		}
		addPrincipal(ResourcePrincipal{
			SID:       user.SID,
			Name:      displayName(user),
			Source:    "group-member",
			GroupName: groupName,
			GroupSID:  strings.TrimSpace(group.SID),
			ViaChain:  strings.TrimSpace(viaChain),
			Enabled:   &enabled,
		}, permission.Rights, permission.Type, permission.RiskLevel, permission.Path)
		groupMembers[groupKey][strings.ToUpper(strings.TrimSpace(user.SID))] = struct{}{}
	}
	addPermissionGroupMember := func(group models.ADGroupRecord, fallbackName string, permission models.Permission) {
		groupKey := addGroup(group, fallbackName, permission)
		groupName := strings.TrimSpace(group.Name)
		if groupName == "" {
			groupName = strings.TrimSpace(fallbackName)
		}
		memberName := strings.TrimSpace(permission.Trustee)
		if memberName == "" {
			memberName = strings.TrimSpace(permission.AccountName)
		}
		if memberName == "" {
			memberName = strings.TrimSpace(permission.TrusteeSID)
		}
		addPrincipal(ResourcePrincipal{
			SID:       permission.TrusteeSID,
			Name:      memberName,
			Source:    "group-member",
			GroupName: groupName,
			GroupSID:  strings.TrimSpace(group.SID),
			ViaChain:  strings.TrimSpace(permission.GroupInheritanceHierarchy),
		}, permission.Rights, permission.Type, permission.RiskLevel, permission.Path)
		groupMembers[groupKey][strings.ToUpper(strings.TrimSpace(permission.TrusteeSID))] = struct{}{}
	}

	for _, permission := range permissions {
		result.ACEs = append(result.ACEs, ResourceACE{
			Path:       permission.Path,
			Trustee:    permission.Trustee,
			TrusteeSID: permission.TrusteeSID,
			Rights:     permission.Rights,
			Type:       permission.Type,
			Inherited:  permission.Inherited,
			RiskLevel:  permission.RiskLevel,
		})

		if sourceGroup := strings.TrimSpace(permission.OriginatingGroup); sourceGroup != "" {
			group := groupRecordForName(groupsByName, sourceGroup)
			if user, isUser := usersBySID[permission.TrusteeSID]; isUser {
				addGroupMember(group, sourceGroup, permission, user, permission.GroupInheritanceHierarchy)
			} else {
				addPermissionGroupMember(group, sourceGroup, permission)
			}
			continue
		}

		if group, isGroup := groupsBySID[strings.ToUpper(strings.TrimSpace(permission.TrusteeSID))]; isGroup {
			addGroup(group, group.Name, permission)
			// Expand the group into its (flattened) member users.
			members, err := service.groupMemberUsers(run.ID, group.SID)
			if err != nil {
				return nil, err
			}
			for _, member := range members {
				addGroupMember(group, group.Name, permission, member.user, member.viaChain)
			}
			continue
		}

		if user, isUser := usersBySID[permission.TrusteeSID]; isUser {
			enabled := user.Enabled
			addPrincipal(ResourcePrincipal{
				SID:     user.SID,
				Name:    displayName(user),
				Source:  "user",
				Enabled: &enabled,
			}, permission.Rights, permission.Type, permission.RiskLevel, permission.Path)
			continue
		}

		// Never silently drop a trustee the snapshot cannot explain: label
		// well-known SIDs with a friendly name, keep everything else as-is.
		addPrincipal(ResourcePrincipal{
			SID:    permission.TrusteeSID,
			Name:   friendlyUnresolvedName(permission.TrusteeSID, permission.Trustee),
			Source: "unresolved",
		}, permission.Rights, permission.Type, permission.RiskLevel, permission.Path)
	}

	for groupKey, principalKey := range groupKeys {
		if index, found := principalIndex[principalKey]; found {
			result.Principals[index].MemberCount = len(groupMembers[groupKey])
		}
	}
	result.Principals = sortResourcePrincipals(result.Principals)

	result.Counts.ACEs = len(result.ACEs)
	result.Counts.Principals = len(result.Principals)
	for _, principal := range result.Principals {
		switch principal.Source {
		case "group":
			result.Counts.Groups++
		case "user":
			result.Counts.Users++
		case "group-member":
			result.Counts.ViaGroups++
		default:
			result.Counts.Unresolved++
		}
	}

	return result, nil
}

// --- Resolution helpers ----------------------------------------------------

func (service *Service) resolveSyncRun(id uuid.UUID) (*models.DirectorySyncRun, error) {
	var run models.DirectorySyncRun
	if id != uuid.Nil {
		err := service.db.First(&run, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSyncRunNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("load sync run: %w", err)
		}
		return &run, nil
	}

	err := service.db.
		Where("status = ?", "completed").
		Order("started_at DESC").
		First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNoCompletedSyncRun
	}
	if err != nil {
		return nil, fmt.Errorf("load latest sync run: %w", err)
	}
	return &run, nil
}

func (service *Service) resolveUser(runID uuid.UUID, principal string) (*models.ADUserRecord, error) {
	var user models.ADUserRecord
	err := service.db.
		Where("run_id = ?", runID).
		Where("sid = ? OR LOWER(sam_account_name) = ? OR LOWER(upn) = ?",
			principal, strings.ToLower(principal), strings.ToLower(principal)).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPrincipalNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolve principal: %w", err)
	}
	return &user, nil
}

// resolveSessions returns either the explicitly requested sessions or the
// latest completed session per distinct root path.
func (service *Service) resolveSessions(ids []uuid.UUID) ([]models.ScanSession, error) {
	if len(ids) > 0 {
		var sessions []models.ScanSession
		if err := service.db.Where("id IN ?", ids).Find(&sessions).Error; err != nil {
			return nil, fmt.Errorf("load sessions: %w", err)
		}
		if len(sessions) != len(ids) {
			return nil, ErrSessionNotFound
		}
		return sessions, nil
	}

	var completed []models.ScanSession
	err := service.db.
		Where("status = ?", "completed").
		Order("started_at DESC").
		Find(&completed).Error
	if err != nil {
		return nil, fmt.Errorf("load sessions: %w", err)
	}
	if len(completed) == 0 {
		return nil, ErrNoScanSessions
	}

	sessions := make([]models.ScanSession, 0, len(completed))
	seenRoots := make(map[string]struct{}, len(completed))
	for _, session := range completed {
		root := strings.ToLower(session.RootPath)
		if _, seen := seenRoots[root]; seen {
			continue
		}
		seenRoots[root] = struct{}{}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

// resolveSessionForPath returns the explicitly requested session, or the
// latest completed session whose root path covers the prefix (or is contained
// in it, so querying a share root still finds a deeper scan).
func (service *Service) resolveSessionForPath(id uuid.UUID, pathPrefix string) (*models.ScanSession, error) {
	if id != uuid.Nil {
		var session models.ScanSession
		err := service.db.First(&session, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("load session: %w", err)
		}
		return &session, nil
	}

	var completed []models.ScanSession
	err := service.db.
		Where("status = ?", "completed").
		Order("started_at DESC").
		Find(&completed).Error
	if err != nil {
		return nil, fmt.Errorf("load sessions: %w", err)
	}
	if len(completed) == 0 {
		return nil, ErrNoScanSessions
	}

	for index := range completed {
		if pathsRelated(completed[index].RootPath, pathPrefix) {
			return &completed[index], nil
		}
	}
	return nil, ErrNoMatchingSession
}

func (service *Service) groupNames(runID uuid.UUID, sids []string) (map[string]string, error) {
	names := make(map[string]string, len(sids))
	if len(sids) == 0 {
		return names, nil
	}

	var groups []models.ADGroupRecord
	err := service.db.
		Where("run_id = ? AND sid IN ?", runID, sids).
		Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("load groups: %w", err)
	}
	for _, group := range groups {
		names[group.SID] = group.Name
	}
	return names, nil
}

func (service *Service) usersBySID(runID uuid.UUID, sids []string) (map[string]models.ADUserRecord, error) {
	users := make(map[string]models.ADUserRecord, len(sids))
	if len(sids) == 0 {
		return users, nil
	}

	var records []models.ADUserRecord
	err := service.db.
		Where("run_id = ? AND sid IN ?", runID, sids).
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("load users: %w", err)
	}
	for _, record := range records {
		users[record.SID] = record
	}
	return users, nil
}

func (service *Service) resourceGroups(runID uuid.UUID, sids, names []string) (map[string]models.ADGroupRecord, map[string]models.ADGroupRecord, error) {
	groupsBySID := make(map[string]models.ADGroupRecord, len(sids))
	groupsByName := make(map[string]models.ADGroupRecord, len(names))
	if len(sids) == 0 && len(names) == 0 {
		return groupsBySID, groupsByName, nil
	}

	var records []models.ADGroupRecord
	query := service.db.Where("run_id = ?", runID)
	if len(sids) > 0 && len(names) > 0 {
		query = query.Where("sid IN ? OR LOWER(name) IN ?", sids, names)
	} else if len(sids) > 0 {
		query = query.Where("sid IN ?", sids)
	} else {
		query = query.Where("LOWER(name) IN ?", names)
	}
	err := query.Find(&records).Error
	if err != nil {
		return nil, nil, fmt.Errorf("load groups: %w", err)
	}

	ambiguousNames := make(map[string]struct{})
	for _, record := range records {
		groupsBySID[strings.ToUpper(strings.TrimSpace(record.SID))] = record
		nameKey := normalizeResourceGroupName(record.Name)
		if nameKey == "" {
			continue
		}
		if _, ambiguous := ambiguousNames[nameKey]; ambiguous {
			continue
		}
		if existing, found := groupsByName[nameKey]; found && !strings.EqualFold(existing.SID, record.SID) {
			delete(groupsByName, nameKey)
			ambiguousNames[nameKey] = struct{}{}
			continue
		}
		groupsByName[nameKey] = record
	}
	return groupsBySID, groupsByName, nil
}

func groupRecordForName(groupsByName map[string]models.ADGroupRecord, name string) models.ADGroupRecord {
	return groupsByName[normalizeResourceGroupName(name)]
}

type memberUser struct {
	user     models.ADUserRecord
	viaChain string
}

// groupMemberUsers lists the snapshot users that are (directly or
// transitively) members of the group, with the membership via-chain.
func (service *Service) groupMemberUsers(runID uuid.UUID, groupSID string) ([]memberUser, error) {
	var memberships []models.ADMembershipRecord
	err := service.db.
		Where("run_id = ? AND group_sid = ?", runID, groupSID).
		Find(&memberships).Error
	if err != nil {
		return nil, fmt.Errorf("load group members: %w", err)
	}
	if len(memberships) == 0 {
		return nil, nil
	}

	memberSIDs := make([]string, 0, len(memberships))
	for _, membership := range memberships {
		memberSIDs = append(memberSIDs, membership.MemberSID)
	}
	users, err := service.usersBySID(runID, memberSIDs)
	if err != nil {
		return nil, err
	}

	members := make([]memberUser, 0, len(memberships))
	for _, membership := range memberships {
		// group->group edges have no ad_users row; the flattened closure
		// already produced the user->group edge separately, so skip them.
		user, isUser := users[membership.MemberSID]
		if !isUser {
			continue
		}
		members = append(members, memberUser{user: user, viaChain: membership.ViaChain})
	}
	sort.Slice(members, func(left, right int) bool { return members[left].user.SID < members[right].user.SID })
	return members, nil
}

// --- Small pure helpers ------------------------------------------------

func displayName(user models.ADUserRecord) string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	if user.SAMAccountName != "" {
		return user.SAMAccountName
	}
	if user.UPN != "" {
		return user.UPN
	}
	return user.SID
}

// friendlyUnresolvedName labels well-known Windows SIDs; other unresolved
// trustees keep the scanner-reported trustee name or the SID itself.
func friendlyUnresolvedName(sid, scannedTrustee string) string {
	if name, known := wellKnownSIDs[sid]; known {
		return name
	}
	if strings.HasPrefix(sid, "S-1-5-32-") {
		return "BUILTIN (" + sid + ")"
	}
	if scannedTrustee != "" {
		return scannedTrustee
	}
	return sid
}

func normalizeResourceGroupName(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.LastIndexAny(value, `\/`); index >= 0 {
		value = value[index+1:]
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func resourceGroupKey(sid, name string) string {
	if sid = strings.ToUpper(strings.TrimSpace(sid)); sid != "" {
		return "sid:" + sid
	}
	return "name:" + normalizeResourceGroupName(name)
}

func resourcePrincipalKey(principal ResourcePrincipal) string {
	identity := strings.ToUpper(strings.TrimSpace(principal.SID))
	if identity == "" {
		identity = strings.ToLower(strings.TrimSpace(principal.Name))
	}
	switch principal.Source {
	case "group":
		return "group|" + resourceGroupKey(principal.SID, principal.Name)
	case "group-member":
		return "group-member|" + resourceGroupKey(principal.GroupSID, principal.GroupName) + "|" + identity
	default:
		return principal.Source + "|" + identity
	}
}

func sortResourcePrincipals(principals []ResourcePrincipal) []ResourcePrincipal {
	groups := make([]ResourcePrincipal, 0)
	membersByGroup := make(map[string][]ResourcePrincipal)
	directUsers := make([]ResourcePrincipal, 0)
	unresolved := make([]ResourcePrincipal, 0)

	for _, principal := range principals {
		switch principal.Source {
		case "group":
			groups = append(groups, principal)
		case "group-member":
			key := resourceGroupKey(principal.GroupSID, principal.GroupName)
			membersByGroup[key] = append(membersByGroup[key], principal)
		case "user":
			directUsers = append(directUsers, principal)
		default:
			unresolved = append(unresolved, principal)
		}
	}

	sort.SliceStable(groups, func(left, right int) bool {
		return resourcePrincipalLess(groups[left], groups[right])
	})
	for key := range membersByGroup {
		sort.SliceStable(membersByGroup[key], func(left, right int) bool {
			return resourcePrincipalLess(membersByGroup[key][left], membersByGroup[key][right])
		})
	}
	sort.SliceStable(directUsers, func(left, right int) bool {
		return resourcePrincipalLess(directUsers[left], directUsers[right])
	})
	sort.SliceStable(unresolved, func(left, right int) bool {
		return resourcePrincipalLess(unresolved[left], unresolved[right])
	})

	result := make([]ResourcePrincipal, 0, len(principals))
	for _, group := range groups {
		result = append(result, group)
		key := resourceGroupKey(group.SID, group.Name)
		result = append(result, membersByGroup[key]...)
		delete(membersByGroup, key)
	}

	// Defensive fallback for historical rows whose group identity could not
	// be matched to a parent. The normal classification path always creates
	// the parent first, but never drop members if legacy evidence is malformed.
	orphanKeys := make([]string, 0, len(membersByGroup))
	for key := range membersByGroup {
		orphanKeys = append(orphanKeys, key)
	}
	sort.Strings(orphanKeys)
	for _, key := range orphanKeys {
		result = append(result, membersByGroup[key]...)
	}

	result = append(result, directUsers...)
	result = append(result, unresolved...)
	return result
}

func resourcePrincipalLess(left, right ResourcePrincipal) bool {
	leftName := strings.ToLower(strings.TrimSpace(left.Name))
	rightName := strings.ToLower(strings.TrimSpace(right.Name))
	if leftName != rightName {
		return leftName < rightName
	}
	return strings.ToUpper(strings.TrimSpace(left.SID)) < strings.ToUpper(strings.TrimSpace(right.SID))
}

var riskRank = map[string]int{"": 0, "low": 1, "medium": 2, "high": 3, "critical": 4}

func highestRisk(current, candidate string) string {
	if riskRank[strings.ToLower(candidate)] > riskRank[strings.ToLower(current)] {
		return candidate
	}
	return current
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}
	return append(values, value)
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

// pathsRelated reports whether one path contains the other (case-insensitive,
// separator-tolerant): a session rooted at C:\Share covers C:\Share\HR, and a
// query for C:\Share is also answerable by a session rooted at C:\Share\HR.
func pathsRelated(rootPath, queryPath string) bool {
	root := normalizePath(rootPath)
	query := normalizePath(queryPath)
	if root == query {
		return true
	}
	return strings.HasPrefix(query, root+"\\") || strings.HasPrefix(root, query+"\\")
}

func normalizePath(path string) string {
	normalized := strings.ToLower(strings.TrimSpace(path))
	normalized = strings.ReplaceAll(normalized, "/", "\\")
	return strings.TrimRight(normalized, "\\")
}
