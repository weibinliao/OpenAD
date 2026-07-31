package ad

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/scanner"
)

func TestPermissionExpanderExpandsGroupsAndExcludesUsers(t *testing.T) {
	rootGroupDN := "CN=Finance,OU=Groups,DC=example,DC=com"
	allowedUserDN := "CN=Alice,OU=Users,DC=example,DC=com"
	excludedUserDN := "CN=Bob,OU=Users,DC=example,DC=com"

	directory := newFakeGroupDirectory(
		models.ADGroup{
			DN:   rootGroupDN,
			Name: "Finance",
			Members: []models.ADPrincipal{
				{DN: allowedUserDN, Name: "Alice Example", SAMAccountName: "alice", SID: "S-1-5-21-100", Type: models.ADObjectTypeUser},
				{DN: excludedUserDN, Name: "Bob Example", SAMAccountName: "bob", SID: "S-1-5-21-101", Type: models.ADObjectTypeUser},
			},
		},
	)

	expander := NewPermissionExpander(
		directory,
		&stubGroupSearcher{groups: []models.ADGroup{{DN: rootGroupDN, Name: "Finance"}}},
		WithPermissionExclusionPatterns(nil, []string{"*Bob*"}),
	)

	expanded, err := expander.Expand(context.Background(), []scanner.Permission{{
		Path:       `C:\Finance`,
		Trustee:    `DOMAIN\Finance`,
		TrusteeSID: "S-1-5-21-group",
		Rights:     "Read",
		Type:       "Allow",
		Source:     "Explicit",
	}})

	require.NoError(t, err)
	require.Len(t, expanded, 1)
	assert.Equal(t, "alice", expanded[0].Trustee)
	assert.Equal(t, "S-1-5-21-100", expanded[0].TrusteeSID)
	assert.Contains(t, expanded[0].Source, "effective via Finance")
}

func TestPermissionExpanderCachesGroupLookupsAndDeduplicatesOutput(t *testing.T) {
	rootGroupDN := "CN=Finance,OU=Groups,DC=example,DC=com"
	directory := newFakeGroupDirectory(
		models.ADGroup{
			DN:   rootGroupDN,
			Name: "Finance",
			Members: []models.ADPrincipal{
				{DN: "CN=Alice,OU=Users,DC=example,DC=com", SAMAccountName: "alice", SID: "S-1-5-21-100", Type: models.ADObjectTypeUser},
			},
		},
	)

	searcher := &stubGroupSearcher{groups: []models.ADGroup{{DN: rootGroupDN, Name: "Finance"}}}
	expander := NewPermissionExpander(directory, searcher)

	input := []scanner.Permission{
		{Path: `C:\Finance`, Trustee: `DOMAIN\Finance`, TrusteeSID: "S-1-5-21-group", Rights: "Read", Type: "Allow"},
		{Path: `C:\Finance`, Trustee: `DOMAIN\Finance`, TrusteeSID: "S-1-5-21-group", Rights: "Read", Type: "Allow"},
	}

	expanded, err := expander.Expand(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, expanded, 1)
	assert.Equal(t, 1, searcher.calls)
}

func TestPermissionExpanderFallsBackToOriginalPermissionWhenAllMembersExcluded(t *testing.T) {
	rootGroupDN := "CN=Finance,OU=Groups,DC=example,DC=com"
	directory := newFakeGroupDirectory(
		models.ADGroup{
			DN:   rootGroupDN,
			Name: "Finance",
			Members: []models.ADPrincipal{
				{DN: "CN=Bob,OU=Users,DC=example,DC=com", Name: "Bob", SAMAccountName: "bob", SID: "S-1-5-21-101", Type: models.ADObjectTypeUser},
			},
		},
	)

	expander := NewPermissionExpander(
		directory,
		&stubGroupSearcher{groups: []models.ADGroup{{DN: rootGroupDN, Name: "Finance"}}},
		WithPermissionExclusionPatterns(nil, []string{"*bob*"}),
	)

	original := scanner.Permission{
		Path:       `C:\Finance`,
		Trustee:    `DOMAIN\Finance`,
		TrusteeSID: "S-1-5-21-group",
		Rights:     "Read",
		Type:       "Allow",
		Source:     "Explicit",
	}

	expanded, err := expander.Expand(context.Background(), []scanner.Permission{original})
	require.NoError(t, err)
	require.Len(t, expanded, 1)
	assert.Equal(t, original.Trustee, expanded[0].Trustee)
	assert.Equal(t, original.TrusteeSID, expanded[0].TrusteeSID)
}

func TestPermissionExpanderResolvesGroupFromSIDWhenSearchMisses(t *testing.T) {
	rootGroupDN := "CN=Finance,OU=Groups,DC=example,DC=com"
	groupSID := "S-1-5-21-999"
	baseDirectory := newFakeGroupDirectory(
		models.ADGroup{
			DN:   rootGroupDN,
			Name: "Finance",
			Members: []models.ADPrincipal{
				{DN: "CN=Alice,OU=Users,DC=example,DC=com", SAMAccountName: "alice", SID: "S-1-5-21-100", Type: models.ADObjectTypeUser},
			},
		},
	)
	directory := &sidResolvingDirectory{
		fakeGroupDirectory: baseDirectory,
		byIdentifier: map[string]models.ADPrincipal{
			strings.ToLower(groupSID): {
				DN:             rootGroupDN,
				Name:           "Finance",
				SAMAccountName: "Finance",
				SID:            groupSID,
				Type:           models.ADObjectTypeGroup,
			},
		},
	}
	baseDirectory.principals[normalizeTestDN(rootGroupDN)] = models.ADPrincipal{
		DN:             rootGroupDN,
		Name:           "Finance",
		SAMAccountName: "Finance",
		SID:            groupSID,
		Type:           models.ADObjectTypeGroup,
	}

	expander := NewPermissionExpander(directory, &stubGroupSearcher{})

	expanded, err := expander.Expand(context.Background(), []scanner.Permission{{
		Path:       `C:\Finance`,
		Trustee:    groupSID,
		TrusteeSID: groupSID,
		Rights:     "Read",
		Type:       "Allow",
		Source:     "Explicit",
	}})

	require.NoError(t, err)
	require.Len(t, expanded, 1)
	assert.Equal(t, "alice", expanded[0].Trustee)
	assert.Equal(t, "S-1-5-21-100", expanded[0].TrusteeSID)
	assert.Contains(t, expanded[0].Source, "effective via Finance")
}

func TestPermissionExpanderPrefersTrusteeSIDForGroupExpansion(t *testing.T) {
	rootGroupDN := "CN=Finance,OU=Groups,DC=example,DC=com"
	groupSID := "S-1-5-21-999"
	baseDirectory := newFakeGroupDirectory(
		models.ADGroup{
			DN:   rootGroupDN,
			Name: "Finance",
			Members: []models.ADPrincipal{
				{DN: "CN=Alice,OU=Users,DC=example,DC=com", SAMAccountName: "alice", SID: "S-1-5-21-100", Type: models.ADObjectTypeUser},
			},
		},
	)
	directory := &sidResolvingDirectory{
		fakeGroupDirectory: baseDirectory,
		byIdentifier: map[string]models.ADPrincipal{
			strings.ToLower(groupSID): {
				DN:             rootGroupDN,
				Name:           "Finance",
				SAMAccountName: "Finance",
				SID:            groupSID,
				Type:           models.ADObjectTypeGroup,
			},
		},
	}

	expander := NewPermissionExpander(directory, &stubGroupSearcher{})

	expanded, err := expander.Expand(context.Background(), []scanner.Permission{{
		Path:       `C:\Finance`,
		Trustee:    `Account Unknown`,
		TrusteeSID: groupSID,
		Rights:     "Read",
		Type:       "Allow",
		Source:     "Explicit",
	}})

	require.NoError(t, err)
	require.Len(t, expanded, 1)
	assert.Equal(t, "alice", expanded[0].Trustee)
	assert.Equal(t, "S-1-5-21-100", expanded[0].TrusteeSID)
	assert.Contains(t, expanded[0].Source, "effective via Finance")
}

func TestPermissionExpanderEnrichesDirectPermissionFromTrusteeSID(t *testing.T) {
	userSID := "S-1-5-21-1234"
	directory := &sidResolvingDirectory{
		fakeGroupDirectory: newFakeGroupDirectory(),
		byIdentifier: map[string]models.ADPrincipal{
			strings.ToLower(userSID): {
				DN:             "CN=Alice,OU=Users,DC=example,DC=com",
				Name:           "Alice Example",
				SAMAccountName: "alice",
				SID:            userSID,
				Email:          "alice@example.com",
				Department:     "Finance",
				Division:       "Corporate",
				Domain:         "EXAMPLE",
				Type:           models.ADObjectTypeUser,
			},
		},
	}

	expander := NewPermissionExpander(directory, &stubGroupSearcher{})

	expanded, err := expander.Expand(context.Background(), []scanner.Permission{{
		Path:       `C:\Finance`,
		Trustee:    userSID,
		TrusteeSID: userSID,
		Rights:     "Read",
		Type:       "Allow",
		Source:     "Explicit",
	}})

	require.NoError(t, err)
	require.Len(t, expanded, 1)
	assert.Equal(t, "EXAMPLE\\alice", expanded[0].Trustee)
	assert.Equal(t, userSID, expanded[0].TrusteeSID)
	assert.Equal(t, "alice", expanded[0].AccountName)
	assert.Equal(t, "alice@example.com", expanded[0].Email)
}

func TestPermissionExpanderDoesNotSearchGroupsAfterCancellation(t *testing.T) {
	searcher := &stubGroupSearcher{}
	expander := NewPermissionExpander(newFakeGroupDirectory(), searcher)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	group, found, err := expander.findGroupForTrustee(ctx, "Finance")

	assert.Nil(t, group)
	assert.False(t, found)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, searcher.calls)
}

type sidResolvingDirectory struct {
	*fakeGroupDirectory
	byIdentifier map[string]models.ADPrincipal
}

func (directory *sidResolvingDirectory) ResolvePrincipal(_ context.Context, identifier string) (*models.ADPrincipal, error) {
	principal, found := directory.byIdentifier[strings.ToLower(strings.TrimSpace(identifier))]
	if !found {
		return nil, nil
	}
	principalCopy := principal
	return &principalCopy, nil
}

type stubGroupSearcher struct {
	groups []models.ADGroup
	calls  int
}

func (searcher *stubGroupSearcher) SearchGroups(_ string, _ int) ([]models.ADGroup, error) {
	searcher.calls++
	return append([]models.ADGroup(nil), searcher.groups...), nil
}
