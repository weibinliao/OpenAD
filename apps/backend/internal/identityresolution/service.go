package identityresolution

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/scanner"
	"gorm.io/gorm"
)

type LiveExpander interface {
	Expand(ctx context.Context, permissions []scanner.Permission) ([]scanner.Permission, error)
}

type Options struct {
	RunID           uuid.UUID
	ConnectionID    uuid.UUID
	LiveExpander    LiveExpander
	LiveUnavailable bool
}

type Metadata struct {
	DirectorySyncRunID       uuid.UUID `json:"directory_sync_run_id,omitempty"`
	Mode                     string    `json:"mode"`
	ResolvedPrincipalCount   int       `json:"resolved_principal_count"`
	UnresolvedPrincipalCount int       `json:"unresolved_principal_count"`
	Warning                  string    `json:"warning,omitempty"`
	Inference                string    `json:"inference,omitempty"`
}

type Result struct {
	Permissions []scanner.Permission `json:"permissions"`
	Metadata    Metadata             `json:"identity_resolution"`
}

type Service struct {
	db      *gorm.DB
	options Options
	mu      sync.RWMutex
	last    Metadata
}

func NewService(db *gorm.DB, options Options) *Service {
	return &Service{db: db, options: options}
}

func (service *Service) Expand(ctx context.Context, permissions []scanner.Permission) ([]scanner.Permission, error) {
	result, err := service.Resolve(ctx, permissions)
	if err != nil {
		return nil, err
	}
	return result.Permissions, nil
}

func (service *Service) Metadata() Metadata {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return service.last
}

func (service *Service) Close() {
	if closer, ok := service.options.LiveExpander.(interface{ Close() }); ok {
		closer.Close()
	}
}

func (service *Service) Resolve(ctx context.Context, permissions []scanner.Permission) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	run, err := service.selectRun()
	if err != nil {
		return Result{}, err
	}

	metadata := Metadata{Mode: "raw"}
	if run != nil {
		metadata.DirectorySyncRunID = run.ID
		metadata.Mode = "snapshot"
	}
	if service.options.LiveUnavailable {
		metadata.Warning = "LDAP supplementation unavailable"
	}

	users, groups, memberships, err := service.loadSnapshot(run)
	if err != nil {
		return Result{}, err
	}

	resolvedPrincipals := make(map[string]struct{})
	allPrincipals := make(map[string]struct{})
	output := make([]scanner.Permission, 0, len(permissions))
	misses := make(map[string][]scanner.Permission)

	for _, permission := range permissions {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		principalKey := permissionPrincipalKey(permission)
		allPrincipals[principalKey] = struct{}{}

		if name, found := wellKnownSIDs[strings.ToUpper(strings.TrimSpace(permission.TrusteeSID))]; found {
			permission.Trustee = name
			permission.AccountType = "well-known"
			permission.ResolutionSource = "windows"
			permission.ResolutionReason = "well_known_sid"
			output = append(output, permission)
			resolvedPrincipals[principalKey] = struct{}{}
			continue
		}

		if user, found := users[strings.ToUpper(strings.TrimSpace(permission.TrusteeSID))]; found {
			output = append(output, permissionForUser(permission, user, "", ""))
			resolvedPrincipals[principalKey] = struct{}{}
			continue
		}

		if group, found := groups[strings.ToUpper(strings.TrimSpace(permission.TrusteeSID))]; found {
			members := memberships[strings.ToUpper(group.SID)]
			added := false
			for _, membership := range members {
				user, isUser := users[strings.ToUpper(membership.MemberSID)]
				if !isUser {
					continue
				}
				hierarchy := strings.TrimSpace(membership.ViaChain)
				if hierarchy == "" {
					hierarchy = group.Name
				}
				output = append(output, permissionForUser(permission, user, group.Name, hierarchy))
				added = true
			}
			if !added {
				permission.Trustee = firstNonEmpty(group.Name, permission.Trustee)
				permission.AccountType = "group"
				permission.ResolutionSource = "snapshot"
				permission.ResolutionReason = ""
				output = append(output, permission)
			}
			resolvedPrincipals[principalKey] = struct{}{}
			continue
		}

		misses[principalKey] = append(misses[principalKey], permission)
	}

	usedLDAP := false
	warnings := make([]string, 0)
	missKeys := make([]string, 0, len(misses))
	for key := range misses {
		missKeys = append(missKeys, key)
	}
	sort.Strings(missKeys)
	for _, key := range missKeys {
		input := misses[key]
		if service.options.LiveExpander == nil {
			reason := "not_in_snapshot"
			if service.options.LiveUnavailable {
				reason = "ldap_unavailable"
			}
			if run == nil {
				reason = "no_snapshot"
			}
			output = append(output, markRaw(input, reason)...)
			continue
		}

		usedLDAP = true
		livePermissions, liveErr := service.options.LiveExpander.Expand(ctx, input)
		if errors.Is(liveErr, context.Canceled) {
			return Result{}, liveErr
		}
		if liveErr != nil {
			warnings = append(warnings, "LDAP supplementation unavailable")
			output = append(output, markRaw(input, "ldap_unavailable")...)
			continue
		}
		if len(livePermissions) == 0 {
			output = append(output, markRaw(input, "ldap_empty_result")...)
			continue
		}
		for _, permission := range livePermissions {
			permission.ResolutionSource = "ldap"
			permission.ResolutionReason = ""
			output = append(output, permission)
		}
		resolvedPrincipals[key] = struct{}{}
	}

	if usedLDAP {
		if run != nil {
			metadata.Mode = "snapshot+ldap"
		} else {
			metadata.Mode = "ldap"
		}
	}
	metadata.ResolvedPrincipalCount = len(resolvedPrincipals)
	metadata.UnresolvedPrincipalCount = len(allPrincipals) - len(resolvedPrincipals)
	if service.options.LiveUnavailable {
		warnings = append(warnings, metadata.Warning)
	}
	metadata.Warning = strings.Join(dedupeStrings(warnings), "; ")
	output = dedupePermissions(output)

	service.mu.Lock()
	service.last = metadata
	service.mu.Unlock()

	return Result{Permissions: output, Metadata: metadata}, nil
}

func (service *Service) selectRun() (*models.DirectorySyncRun, error) {
	if service == nil || service.db == nil {
		return nil, nil
	}

	query := service.db.Where("status = ?", "completed")
	if service.options.RunID != uuid.Nil {
		query = query.Where("id = ?", service.options.RunID)
	} else if service.options.ConnectionID != uuid.Nil {
		query = query.Where("connection_id = ?", service.options.ConnectionID)
	} else {
		return nil, nil
	}

	var run models.DirectorySyncRun
	err := query.Order("started_at DESC").First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select directory snapshot: %w", err)
	}
	return &run, nil
}

func (service *Service) loadSnapshot(run *models.DirectorySyncRun) (map[string]models.ADUserRecord, map[string]models.ADGroupRecord, map[string][]models.ADMembershipRecord, error) {
	users := make(map[string]models.ADUserRecord)
	groups := make(map[string]models.ADGroupRecord)
	memberships := make(map[string][]models.ADMembershipRecord)
	if run == nil || service.db == nil {
		return users, groups, memberships, nil
	}

	var userRecords []models.ADUserRecord
	if err := service.db.Where("run_id = ?", run.ID).Find(&userRecords).Error; err != nil {
		return nil, nil, nil, fmt.Errorf("load snapshot users: %w", err)
	}
	for _, user := range userRecords {
		users[strings.ToUpper(strings.TrimSpace(user.SID))] = user
	}

	var groupRecords []models.ADGroupRecord
	if err := service.db.Where("run_id = ?", run.ID).Find(&groupRecords).Error; err != nil {
		return nil, nil, nil, fmt.Errorf("load snapshot groups: %w", err)
	}
	for _, group := range groupRecords {
		groups[strings.ToUpper(strings.TrimSpace(group.SID))] = group
	}

	var membershipRecords []models.ADMembershipRecord
	if err := service.db.Where("run_id = ?", run.ID).Find(&membershipRecords).Error; err != nil {
		return nil, nil, nil, fmt.Errorf("load snapshot memberships: %w", err)
	}
	for _, membership := range membershipRecords {
		key := strings.ToUpper(strings.TrimSpace(membership.GroupSID))
		memberships[key] = append(memberships[key], membership)
	}
	for key := range memberships {
		sort.SliceStable(memberships[key], func(i, j int) bool {
			return strings.ToLower(memberships[key][i].MemberSID) < strings.ToLower(memberships[key][j].MemberSID)
		})
	}
	return users, groups, memberships, nil
}

func permissionForUser(permission scanner.Permission, user models.ADUserRecord, originatingGroup, hierarchy string) scanner.Permission {
	permission.TrusteeSID = user.SID
	permission.Trustee = firstNonEmpty(user.DisplayName, qualifiedAccountName(user), user.UPN, user.SID)
	permission.AccountType = "user"
	permission.AccountName = user.SAMAccountName
	permission.FirstName = user.FirstName
	permission.LastName = user.LastName
	permission.Email = user.Email
	permission.Department = user.Department
	permission.Division = user.Division
	permission.Domain = user.Domain
	permission.OriginatingGroup = originatingGroup
	permission.GroupInheritanceHierarchy = hierarchy
	permission.ResolutionSource = "snapshot"
	permission.ResolutionReason = ""
	return permission
}

func qualifiedAccountName(user models.ADUserRecord) string {
	if strings.TrimSpace(user.Domain) != "" && strings.TrimSpace(user.SAMAccountName) != "" {
		return strings.TrimSpace(user.Domain) + `\` + strings.TrimSpace(user.SAMAccountName)
	}
	return strings.TrimSpace(user.SAMAccountName)
}

func markRaw(permissions []scanner.Permission, reason string) []scanner.Permission {
	result := make([]scanner.Permission, 0, len(permissions))
	for _, permission := range permissions {
		permission.ResolutionSource = "raw"
		permission.ResolutionReason = reason
		result = append(result, permission)
	}
	return result
}

func permissionPrincipalKey(permission scanner.Permission) string {
	key := strings.TrimSpace(permission.TrusteeSID)
	if key == "" {
		key = strings.TrimSpace(permission.Trustee)
	}
	return strings.ToUpper(key)
}

func dedupePermissions(permissions []scanner.Permission) []scanner.Permission {
	seen := make(map[string]struct{}, len(permissions))
	result := make([]scanner.Permission, 0, len(permissions))
	for _, permission := range permissions {
		key := strings.ToLower(strings.Join([]string{
			permission.Path,
			permission.TrusteeSID,
			permission.Rights,
			permission.Type,
			strconv.FormatBool(permission.Inherited),
			permission.OriginatingGroup,
			permission.GroupInheritanceHierarchy,
			permission.ResolutionSource,
		}, "|"))
		if _, found := seen[key]; found {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, permission)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		if _, found := seen[key]; found {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, strings.TrimSpace(value))
	}
	return result
}

var wellKnownSIDs = map[string]string{
	"S-1-0-0":      "Nobody",
	"S-1-1-0":      "Everyone",
	"S-1-3-0":      "CREATOR OWNER",
	"S-1-3-1":      "CREATOR GROUP",
	"S-1-5-4":      `NT AUTHORITY\INTERACTIVE`,
	"S-1-5-7":      `NT AUTHORITY\ANONYMOUS LOGON`,
	"S-1-5-9":      `NT AUTHORITY\ENTERPRISE DOMAIN CONTROLLERS`,
	"S-1-5-11":     `NT AUTHORITY\Authenticated Users`,
	"S-1-5-18":     `NT AUTHORITY\SYSTEM`,
	"S-1-5-19":     `NT AUTHORITY\LOCAL SERVICE`,
	"S-1-5-20":     `NT AUTHORITY\NETWORK SERVICE`,
	"S-1-5-32-544": `BUILTIN\Administrators`,
	"S-1-5-32-545": `BUILTIN\Users`,
	"S-1-5-32-546": `BUILTIN\Guests`,
	"S-1-5-32-547": `BUILTIN\Power Users`,
	"S-1-5-32-551": `BUILTIN\Backup Operators`,
}
