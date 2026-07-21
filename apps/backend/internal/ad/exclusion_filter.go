package ad

import (
	"path/filepath"
	"strings"

	"github.com/weibinliao/OpenAD/internal/models"
)

type ExclusionFilter struct {
	groupPatterns []string
	userPatterns  []string
	builtInGroups []string
}

func NewExclusionFilter() *ExclusionFilter {
	return &ExclusionFilter{
		groupPatterns: []string{},
		userPatterns:  []string{},
		builtInGroups: []string{
			"BUILTIN\\Administrators",
			"BUILTIN\\Users",
			"BUILTIN\\Guests",
			"NT AUTHORITY\\SYSTEM",
			"NT AUTHORITY\\Authenticated Users",
		},
	}
}

func (f *ExclusionFilter) AddPattern(pattern string) {
	f.AddGroupPattern(pattern)
}

func (f *ExclusionFilter) AddGroupPattern(pattern string) {
	if f == nil {
		return
	}

	f.groupPatterns = append(f.groupPatterns, strings.TrimSpace(pattern))
}

func (f *ExclusionFilter) AddUserPattern(pattern string) {
	if f == nil {
		return
	}

	f.userPatterns = append(f.userPatterns, strings.TrimSpace(pattern))
}

func (f *ExclusionFilter) ShouldExcludeTrustee(trustee string) bool {
	normalizedTrustee := strings.TrimSpace(trustee)
	return f.matchesAny(normalizedTrustee, append([]string(nil), f.builtInGroups...)) ||
		f.matchesAny(normalizedTrustee, f.groupPatterns) ||
		f.matchesAny(normalizedTrustee, f.userPatterns)
}

func (f *ExclusionFilter) ShouldExclude(trustee string) bool {
	return f.ShouldExcludeTrustee(trustee)
}

func (f *ExclusionFilter) ShouldExcludeGroup(principal models.ADPrincipal) bool {
	return f.matchesAnyInPrincipal(principal, append(append([]string(nil), f.builtInGroups...), f.groupPatterns...))
}

func (f *ExclusionFilter) ShouldExcludeUser(principal models.ADPrincipal) bool {
	return f.matchesAnyInPrincipal(principal, f.userPatterns)
}

func (f *ExclusionFilter) matchesAny(value string, patterns []string) bool {
	normalizedValue := strings.TrimSpace(value)
	for _, pattern := range patterns {
		normalizedPattern := strings.TrimSpace(pattern)
		if normalizedPattern == "" {
			continue
		}

		if matched, _ := filepath.Match(normalizedPattern, normalizedValue); matched {
			return true
		}

		if matched, _ := filepath.Match(strings.ToLower(normalizedPattern), strings.ToLower(normalizedValue)); matched {
			return true
		}

		// Allow DOMAIN\user-pattern to match samAccountName values.
		if index := strings.LastIndex(normalizedPattern, `\`); index >= 0 && index < len(normalizedPattern)-1 {
			shortPattern := normalizedPattern[index+1:]
			if matched, _ := filepath.Match(shortPattern, normalizedValue); matched {
				return true
			}

			if matched, _ := filepath.Match(strings.ToLower(shortPattern), strings.ToLower(normalizedValue)); matched {
				return true
			}
		}
	}

	return false
}

func (f *ExclusionFilter) matchesAnyInPrincipal(principal models.ADPrincipal, patterns []string) bool {
	for _, candidate := range principalDisplayValues(principal) {
		if f.matchesAny(candidate, patterns) {
			return true
		}
	}

	return false
}

func principalDisplayValues(principal models.ADPrincipal) []string {
	return []string{
		principal.DN,
		principal.Name,
		principal.SAMAccountName,
		principal.SID,
	}
}
