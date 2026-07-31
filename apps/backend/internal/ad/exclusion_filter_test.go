package ad

import (
	"testing"

	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestExclusionFilterExcludesBuiltInGroupsCaseInsensitively(t *testing.T) {
	filter := NewExclusionFilter()

	assert.True(t, filter.ShouldExclude("builtin\\administrators"))
	assert.True(t, filter.ShouldExclude("NT AUTHORITY\\SYSTEM"))
	assert.False(t, filter.ShouldExclude("EXAMPLE\\FinanceTeam"))
}

func TestExclusionFilterMatchesCustomPatterns(t *testing.T) {
	filter := NewExclusionFilter()
	filter.AddPattern("EXAMPLE\\svc-*")
	filter.AddPattern("example\\audit-*")

	assert.True(t, filter.ShouldExclude("EXAMPLE\\svc-backup"))
	assert.True(t, filter.ShouldExclude("EXAMPLE\\AUDIT-readers"))
	assert.False(t, filter.ShouldExclude("EXAMPLE\\engineering"))
}

func TestExclusionFilterSeparatesGroupAndUserPatterns(t *testing.T) {
	filter := NewExclusionFilter()
	filter.AddGroupPattern("EXAMPLE\\Finance*")
	filter.AddUserPattern("EXAMPLE\\temp-*")

	assert.True(t, filter.ShouldExcludeGroup(models.ADPrincipal{DN: "CN=Finance,OU=Groups,DC=example,DC=com", Name: "EXAMPLE\\FinanceTeam", Type: models.ADObjectTypeGroup}))
	assert.True(t, filter.ShouldExcludeUser(models.ADPrincipal{DN: "CN=Temp,OU=Users,DC=example,DC=com", SAMAccountName: "temp-01", Type: models.ADObjectTypeUser}))
	assert.False(t, filter.ShouldExcludeGroup(models.ADPrincipal{DN: "CN=Audit,OU=Groups,DC=example,DC=com", Name: "EXAMPLE\\AuditTeam", Type: models.ADObjectTypeGroup}))
}

func TestExclusionFilterDoesNotTurnQualifiedWildcardIntoGlobalWildcard(t *testing.T) {
	filter := NewExclusionFilter()
	filter.AddGroupPattern(`NT AUTHORITY\*`)

	assert.True(t, filter.ShouldExclude(`NT AUTHORITY\SYSTEM`))
	assert.False(t, filter.ShouldExclude(`S-1-5-21-1-2-3-1001`))
	assert.False(t, filter.ShouldExclude(`CORP\Finance`))
	assert.False(t, filter.ShouldExclude(`alice`))
}
