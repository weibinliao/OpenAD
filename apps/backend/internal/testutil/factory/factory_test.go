package factory

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScanSessionAppliesDefaultsAndOverrides(t *testing.T) {
	finishedAt := time.Now().UTC()
	session := NewScanSession(ScanSessionParams{
		RootPath:         `C:\Finance`,
		Status:           "failed",
		ItemsScanned:     7,
		PermissionCount:  2,
		FinishedAt:       &finishedAt,
		IncludeInherited: true,
	})

	assert.NotEqual(t, uuid.Nil, session.ID)
	assert.Equal(t, `C:\Finance`, session.RootPath)
	assert.Equal(t, "failed", session.Status)
	assert.Equal(t, 7, session.ItemsScanned)
	assert.Equal(t, 2, session.PermissionCount)
	assert.True(t, session.IncludeInherited)
	require.NotNil(t, session.FinishedAt)
	assert.Equal(t, finishedAt, *session.FinishedAt)
}

func TestNewPermissionCreatesUniqueDefaults(t *testing.T) {
	firstPermission := NewPermission(PermissionParams{})
	secondPermission := NewPermission(PermissionParams{})

	assert.NotEqual(t, firstPermission.ID, secondPermission.ID)
	assert.NotEmpty(t, firstPermission.Path)
	assert.NotEmpty(t, firstPermission.Trustee)
	assert.NotEmpty(t, firstPermission.TrusteeSID)
	assert.Equal(t, "Read", firstPermission.Rights)
	assert.Equal(t, "Allow", firstPermission.Type)
}

func TestNewScanResponseBuildsDefaultPermissionSet(t *testing.T) {
	response := NewScanResponse(ScanResponseParams{})

	assert.NotEmpty(t, response.SessionID)
	assert.NotEmpty(t, response.RootPath)
	assert.False(t, response.StartedAt.IsZero())
	assert.False(t, response.FinishedAt.IsZero())
	require.Len(t, response.Permissions, 1)
	assert.Equal(t, 1, response.PermissionCount)
	assert.Equal(t, 1, response.ItemsScanned)
}

func TestNewADUserCopiesGroupsAndAppliesDefaults(t *testing.T) {
	user := NewADUser(ADUserParams{Groups: []string{"CN=Finance,OU=Groups,DC=example,DC=com"}})

	assert.NotEmpty(t, user.DN)
	assert.NotEmpty(t, user.Username)
	assert.NotEmpty(t, user.DisplayName)
	assert.NotEmpty(t, user.Email)
	assert.Equal(t, []string{"CN=Finance,OU=Groups,DC=example,DC=com"}, user.Groups)
}
