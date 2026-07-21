package ad

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/scanner"
)

type GroupSearcher interface {
	SearchGroups(query string, limit int) ([]models.ADGroup, error)
}

type PrincipalResolver interface {
	ResolvePrincipal(ctx context.Context, identifier string) (*models.ADPrincipal, error)
}

type PermissionExpander struct {
	directory       GroupDirectory
	searcher        GroupSearcher
	principalLookup PrincipalResolver
	resolver        *GroupResolver
	filter          *ExclusionFilter
	closer          interface{ Close() }
	groupCache      map[string]groupLookupResult
	resolutionCache map[string]*models.ADGroupResolution
	principalCache  map[string]*models.ADPrincipal
}

type PermissionExpanderOption func(*PermissionExpander)
type groupLookupResult struct {
	group *models.ADGroup
	found bool
}

func NewPermissionExpander(directory GroupDirectory, searcher GroupSearcher, options ...PermissionExpanderOption) *PermissionExpander {
	expander := &PermissionExpander{
		directory:       directory,
		searcher:        searcher,
		filter:          NewExclusionFilter(),
		groupCache:      make(map[string]groupLookupResult),
		resolutionCache: make(map[string]*models.ADGroupResolution),
		principalCache:  make(map[string]*models.ADPrincipal),
	}

	if resolver, ok := any(directory).(PrincipalResolver); ok {
		expander.principalLookup = resolver
	}

	if closer, ok := any(directory).(interface{ Close() }); ok {
		expander.closer = closer
	}

	expander.resolver = NewGroupResolver(directory, WithExclusionFilter(expander.filter))

	for _, option := range options {
		if option != nil {
			option(expander)
		}
	}

	if expander.filter == nil {
		expander.filter = NewExclusionFilter()
		expander.resolver = NewGroupResolver(directory, WithExclusionFilter(expander.filter))
	}

	return expander
}

func WithPermissionExclusionPatterns(groupPatterns, userPatterns []string) PermissionExpanderOption {
	return func(expander *PermissionExpander) {
		if expander.filter == nil {
			expander.filter = NewExclusionFilter()
		}

		for _, pattern := range groupPatterns {
			expander.filter.AddGroupPattern(pattern)
		}

		for _, pattern := range userPatterns {
			expander.filter.AddUserPattern(pattern)
		}

		if expander.resolver == nil {
			expander.resolver = NewGroupResolver(expander.directory, WithExclusionFilter(expander.filter))
			return
		}

		expander.resolver = NewGroupResolver(expander.directory, WithExclusionFilter(expander.filter))
	}
}

func (expander *PermissionExpander) Expand(ctx context.Context, permissions []scanner.Permission) ([]scanner.Permission, error) {
	if expander == nil {
		return append([]scanner.Permission(nil), permissions...), nil
	}

	expanded := make([]scanner.Permission, 0, len(permissions))
	seen := make(map[string]struct{}, len(permissions))

	for _, permission := range permissions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		group, found, err := expander.findGroupForPermission(ctx, permission)
		if err != nil {
			return nil, err
		}

		if !found {
			if expander.filter != nil && expander.filter.ShouldExcludeTrustee(permission.Trustee) {
				continue
			}

			appendUniquePermission(&expanded, seen, expander.enrichDirectPermission(ctx, permission))
			continue
		}

		if expander.filter != nil && expander.filter.ShouldExcludeGroup(models.ADPrincipal{
			DN:   group.DN,
			Name: group.Name,
			Type: models.ADObjectTypeGroup,
		}) {
			continue
		}

		resolution, err := expander.resolveGroupMembers(ctx, group.DN)
		if err != nil {
			return nil, err
		}

		if len(resolution.Members) == 0 {
			appendUniquePermission(&expanded, seen, expander.enrichDirectPermission(ctx, permission))
			continue
		}

		added := 0
		for _, member := range resolution.Members {
			if expander.filter != nil && expander.filter.ShouldExcludeUser(member.ADPrincipal) {
				continue
			}

			appendUniquePermission(&expanded, seen, expander.permissionForMember(permission, group, member))
			added++
		}

		// Keep the original entry when every expanded user is filtered out.
		if added == 0 {
			appendUniquePermission(&expanded, seen, expander.enrichDirectPermission(ctx, permission))
		}
	}

	return expanded, nil
}

func (expander *PermissionExpander) findGroupForPermission(ctx context.Context, permission scanner.Permission) (*models.ADGroup, bool, error) {
	for _, candidate := range permissionLookupTerms(permission) {
		group, found, err := expander.findGroupForTrustee(ctx, candidate)
		if err != nil {
			return nil, false, err
		}
		if found {
			return group, true, nil
		}
	}

	return nil, false, nil
}

func (expander *PermissionExpander) Close() {
	if expander == nil || expander.closer == nil {
		return
	}

	expander.closer.Close()
	expander.closer = nil
}

func (expander *PermissionExpander) findGroupForTrustee(ctx context.Context, trustee string) (*models.ADGroup, bool, error) {
	_ = ctx

	cacheKey := strings.ToLower(strings.TrimSpace(trustee))
	if result, found := expander.groupCache[cacheKey]; found {
		return result.group, result.found, nil
	}

	if expander.searcher != nil {
		candidates := trusteeSearchTerms(trustee)
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}

			groups, err := expander.searcher.SearchGroups(candidate, 10)
			if err != nil {
				return nil, false, err
			}

			if group := pickBestGroup(trustee, groups); group != nil {
				expander.groupCache[cacheKey] = groupLookupResult{group: group, found: true}
				return group, true, nil
			}
		}
	}

	principal, err := expander.resolvePrincipal(ctx, trustee)
	if err != nil {
		return nil, false, err
	}

	if principal != nil && principal.Type == models.ADObjectTypeGroup && strings.TrimSpace(principal.DN) != "" && expander.directory != nil {
		group, groupErr := expander.directory.GetGroup(ctx, principal.DN)
		if groupErr != nil {
			return nil, false, groupErr
		}
		if group != nil {
			expander.groupCache[cacheKey] = groupLookupResult{group: group, found: true}
			return group, true, nil
		}
	}

	expander.groupCache[cacheKey] = groupLookupResult{found: false}
	return nil, false, nil
}

func (expander *PermissionExpander) permissionForMember(permission scanner.Permission, group *models.ADGroup, member models.ADResolvedMember) scanner.Permission {
	expanded := permission
	expanded.Trustee = displayTrustee(member.ADPrincipal)
	expanded.TrusteeSID = permissionIdentifier(member.ADPrincipal)
	expanded.Source = buildEffectiveSource(permission.Source, group)
	expanded.AccountName = strings.TrimSpace(member.SAMAccountName)
	expanded.FirstName = strings.TrimSpace(member.FirstName)
	expanded.LastName = strings.TrimSpace(member.LastName)
	expanded.Email = strings.TrimSpace(member.Email)
	expanded.Department = strings.TrimSpace(member.Department)
	expanded.Division = strings.TrimSpace(member.Division)
	expanded.Domain = strings.TrimSpace(member.Domain)
	expanded.OriginatingGroup = displayGroup(group)
	expanded.GroupInheritanceHierarchy = buildInheritanceHierarchy(group, member.Path)
	return expanded
}

func displayTrustee(principal models.ADPrincipal) string {
	if principal.Domain != "" && principal.SAMAccountName != "" {
		return fmt.Sprintf("%s\\%s", principal.Domain, principal.SAMAccountName)
	}
	if principal.SAMAccountName != "" {
		return principal.SAMAccountName
	}

	if principal.Name != "" {
		return principal.Name
	}

	return principal.DN
}

func permissionIdentifier(principal models.ADPrincipal) string {
	if principal.SID != "" {
		return principal.SID
	}

	if principal.DN != "" {
		return principal.DN
	}

	return displayTrustee(principal)
}

func buildEffectiveSource(source string, group *models.ADGroup) string {
	groupLabel := ""
	if group != nil {
		groupLabel = group.DN
		if group.Name != "" {
			groupLabel = group.Name
		}
	}

	if strings.TrimSpace(source) == "" {
		return fmt.Sprintf("effective via %s", groupLabel)
	}

	return fmt.Sprintf("%s; effective via %s", strings.TrimSpace(source), groupLabel)
}

func displayGroup(group *models.ADGroup) string {
	if group == nil {
		return ""
	}
	if strings.TrimSpace(group.Name) != "" {
		return strings.TrimSpace(group.Name)
	}
	return dnLabel(group.DN)
}

func buildInheritanceHierarchy(group *models.ADGroup, path []string) string {
	labels := make([]string, 0, len(path))
	for _, value := range path {
		label := dnLabel(value)
		if label != "" {
			labels = append(labels, label)
		}
	}

	if len(labels) == 0 {
		if fallback := displayGroup(group); fallback != "" {
			return fallback
		}
		return ""
	}

	if fallback := displayGroup(group); fallback != "" && !strings.EqualFold(labels[0], fallback) {
		labels = append([]string{fallback}, labels...)
	}

	return strings.Join(dedupeStrings(labels), " > ")
}

func dnLabel(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) > 3 && (strings.HasPrefix(strings.ToUpper(part), "CN=") || strings.HasPrefix(strings.ToUpper(part), "OU=")) {
			return strings.TrimSpace(part[3:])
		}
	}
	return trimmed
}

func trusteeSearchTerms(trustee string) []string {
	trimmed := strings.TrimSpace(trustee)
	if trimmed == "" {
		return nil
	}

	terms := []string{trimmed}

	if index := strings.LastIndex(trimmed, "\\"); index >= 0 && index < len(trimmed)-1 {
		terms = append(terms, trimmed[index+1:])
	}

	if index := strings.LastIndex(trimmed, "@"); index >= 0 && index < len(trimmed)-1 {
		terms = append(terms, trimmed[:index])
	}

	return dedupeStrings(terms)
}

func permissionLookupTerms(permission scanner.Permission) []string {
	return dedupeStrings([]string{
		strings.TrimSpace(permission.TrusteeSID),
		strings.TrimSpace(permission.Trustee),
	})
}

func pickBestGroup(trustee string, groups []models.ADGroup) *models.ADGroup {
	if len(groups) == 0 {
		return nil
	}

	trimmed := strings.TrimSpace(trustee)
	shortName := trimmed
	if index := strings.LastIndex(trimmed, "\\"); index >= 0 && index < len(trimmed)-1 {
		shortName = trimmed[index+1:]
	}

	for index := range groups {
		group := groups[index]
		if strings.EqualFold(group.Name, trimmed) || strings.EqualFold(group.Name, shortName) {
			return &groups[index]
		}

		if strings.EqualFold(group.DN, trimmed) {
			return &groups[index]
		}
	}

	return &groups[0]
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))

	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}

		if _, found := seen[normalized]; found {
			continue
		}

		seen[normalized] = struct{}{}
		result = append(result, value)
	}

	return result
}

func (expander *PermissionExpander) resolveGroupMembers(ctx context.Context, groupDN string) (*models.ADGroupResolution, error) {
	cacheKey := strings.ToLower(strings.TrimSpace(groupDN))
	if resolution, found := expander.resolutionCache[cacheKey]; found {
		return resolution, nil
	}

	resolution, err := expander.resolver.ResolveGroupMembers(ctx, groupDN)
	if err != nil {
		return nil, err
	}

	expander.resolutionCache[cacheKey] = resolution
	return resolution, nil
}

func (expander *PermissionExpander) enrichDirectPermission(ctx context.Context, permission scanner.Permission) scanner.Permission {
	var principal *models.ADPrincipal
	for _, candidate := range permissionLookupTerms(permission) {
		resolved, err := expander.resolvePrincipal(ctx, candidate)
		if err != nil {
			return permission
		}
		if resolved != nil {
			principal = resolved
			break
		}
	}

	if principal == nil {
		return permission
	}

	enriched := permission
	enriched.AccountName = firstNonEmptyString(strings.TrimSpace(principal.SAMAccountName), permission.AccountName)
	enriched.FirstName = firstNonEmptyString(strings.TrimSpace(principal.FirstName), permission.FirstName)
	enriched.LastName = firstNonEmptyString(strings.TrimSpace(principal.LastName), permission.LastName)
	enriched.Email = firstNonEmptyString(strings.TrimSpace(principal.Email), permission.Email)
	enriched.Department = firstNonEmptyString(strings.TrimSpace(principal.Department), permission.Department)
	enriched.Division = firstNonEmptyString(strings.TrimSpace(principal.Division), permission.Division)
	enriched.Domain = firstNonEmptyString(strings.TrimSpace(principal.Domain), permission.Domain)
	if strings.TrimSpace(principal.SID) != "" {
		enriched.TrusteeSID = firstNonEmptyString(strings.TrimSpace(principal.SID), permission.TrusteeSID)
	}
	if display := displayTrustee(*principal); display != "" && (strings.EqualFold(enriched.Trustee, permission.Trustee) || isSIDString(permission.Trustee)) {
		enriched.Trustee = display
	}
	if enriched.AccountName != "" && strings.EqualFold(enriched.Trustee, permission.Trustee) {
		enriched.Trustee = displayTrustee(*principal)
	}
	return enriched
}

func (expander *PermissionExpander) resolvePrincipal(ctx context.Context, trustee string) (*models.ADPrincipal, error) {
	if expander == nil || expander.principalLookup == nil {
		return nil, nil
	}

	cacheKey := strings.ToLower(strings.TrimSpace(trustee))
	if cached, found := expander.principalCache[cacheKey]; found {
		return cached, nil
	}

	for _, candidate := range trusteeSearchTerms(trustee) {
		principal, err := expander.principalLookup.ResolvePrincipal(ctx, candidate)
		if err != nil {
			return nil, err
		}
		if principal != nil {
			expander.principalCache[cacheKey] = principal
			return principal, nil
		}
	}

	expander.principalCache[cacheKey] = nil
	return nil, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func appendUniquePermission(permissions *[]scanner.Permission, seen map[string]struct{}, permission scanner.Permission) {
	key := strings.ToLower(strings.Join([]string{
		permission.Path,
		permission.Trustee,
		permission.TrusteeSID,
		permission.Rights,
		permission.Type,
		strconv.FormatBool(permission.Inherited),
		permission.Source,
	}, "|"))
	if _, found := seen[key]; found {
		return
	}

	seen[key] = struct{}{}
	*permissions = append(*permissions, permission)
}
