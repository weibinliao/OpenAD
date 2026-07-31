package groupexport

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/xuri/excelize/v2"
)

func TestCSVIncludesUTF8BOMAndHeadersForEmptyGroup(t *testing.T) {
	exporter := NewExporter()
	data, err := exporter.CSV(models.ADGroup{Name: "Finance", DN: "CN=Finance,DC=corp,DC=test"}, nil)

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(data), 3)
	assert.Equal(t, []byte{0xEF, 0xBB, 0xBF}, data[:3])
	assert.Contains(t, string(data), "Membership Path")
	assert.Contains(t, string(data), "Group Name")
}

func TestXLSXUsesMembersSheetAndWritesMemberFields(t *testing.T) {
	exporter := NewExporter()
	rows := []Row{{
		GroupName: "Finance", GroupDN: "CN=Finance,DC=corp,DC=test", MemberType: "user",
		DisplayName: "Alice Adams", SAMAccountName: "alice", Email: "alice@corp.test",
		Department: "Finance", Division: "Operations", Domain: "CORP",
		SID: "S-1-5-21-1-2-3-1001", MemberDN: "CN=Alice,DC=corp,DC=test",
		Membership: "direct", Depth: 0, MembershipPath: "Finance > Alice Adams",
	}}

	data, err := exporter.XLSX(models.ADGroup{Name: "Finance"}, rows)
	require.NoError(t, err)
	book, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer book.Close()
	assert.Equal(t, "Members", book.GetSheetName(0))
	value, err := book.GetCellValue("Members", "D2")
	require.NoError(t, err)
	assert.Equal(t, "Alice Adams", value)
}

func TestRowsFromResolutionPreservesDirectAndNestedScope(t *testing.T) {
	group := models.ADGroup{Name: "Finance", DN: "CN=Finance,DC=corp,DC=test"}
	resolution := models.ADGroupResolution{Members: []models.ADResolvedMember{
		{ADPrincipal: models.ADPrincipal{DN: "CN=Alice,DC=corp,DC=test", Name: "Alice", SAMAccountName: "alice", SID: "S-1-a", Type: models.ADObjectTypeUser}, Depth: 0, Path: []string{group.DN}},
		{ADPrincipal: models.ADPrincipal{DN: "CN=Bob,DC=corp,DC=test", Name: "Bob", SAMAccountName: "bob", SID: "S-1-b", Type: models.ADObjectTypeUser}, Depth: 2, Path: []string{group.DN, "CN=Helpdesk,DC=corp,DC=test"}},
	}}

	rows := RowsFromResolution(group, resolution)

	require.Len(t, rows, 2)
	assert.Equal(t, "direct", rows[0].Membership)
	assert.Equal(t, "nested", rows[1].Membership)
	assert.Equal(t, 2, rows[1].Depth)
	assert.Contains(t, rows[1].MembershipPath, "Helpdesk")
}

func TestRowsFromDirectMembersUsesOnlyGroupMembers(t *testing.T) {
	group := models.ADGroup{
		Name: "Finance",
		DN:   "CN=Finance,DC=corp,DC=test",
		Members: []models.ADPrincipal{{
			DN: "CN=Alice,DC=corp,DC=test", Name: "Alice", SAMAccountName: "alice", SID: "S-1-a", Type: models.ADObjectTypeUser,
		}},
	}

	rows := RowsFromDirectMembers(group)

	require.Len(t, rows, 1)
	assert.Equal(t, "direct", rows[0].Membership)
	assert.Zero(t, rows[0].Depth)
	assert.Equal(t, "alice", rows[0].SAMAccountName)
}
