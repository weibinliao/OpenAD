package comparison

import (
	"testing"

	"github.com/weibinliao/OpenAD/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectChanges_MapsPermissionFields(t *testing.T) {
	baselineSession := &models.ScanSession{ID: uuid.New()}
	currentSession := &models.ScanSession{ID: uuid.New()}
	engine := NewComparisonEngine(baselineSession, currentSession)

	baselinePerms := []models.Permission{
		newPermission(`C:\removed`, "DOMAIN\\removed", "S-1-5-21-100", "Read"),
		newPermission(`C:\modified`, "DOMAIN\\modified", "S-1-5-21-200", "Read"),
	}
	currentPerms := []models.Permission{
		newPermission(`C:\added`, "DOMAIN\\added", "S-1-5-21-300", "Write"),
		newPermission(`C:\modified`, "DOMAIN\\modified", "S-1-5-21-200", "Full Control"),
	}

	report, err := engine.DetectChanges(baselinePerms, currentPerms)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, baselineSession.ID.String(), report.BaselineID)
	assert.Equal(t, currentSession.ID.String(), report.CurrentID)
	assert.Len(t, report.Changes, 3)
	assert.Equal(t, 3, report.ChangesCount)

	changesByKey := make(map[string]models.PermissionChange, len(report.Changes))
	for _, change := range report.Changes {
		changesByKey[change.ChangeType+"|"+change.Path] = change
		assert.NotEqual(t, uuid.Nil, change.ID)
		assert.Equal(t, currentSession.ID, change.ScanSessionID)
		assert.False(t, change.DetectedAt.IsZero())
	}

	assert.Equal(t, models.PermissionChange{
		ID:            changesByKey["added|C:\\added"].ID,
		ScanSessionID: currentSession.ID,
		Path:          `C:\added`,
		Trustee:       `DOMAIN\added`,
		TrusteeSID:    "S-1-5-21-300",
		ChangeType:    "added",
		CurrentRights: "Write",
		DetectedAt:    changesByKey["added|C:\\added"].DetectedAt,
	}, changesByKey["added|C:\\added"])

	assert.Equal(t, models.PermissionChange{
		ID:             changesByKey["removed|C:\\removed"].ID,
		ScanSessionID:  currentSession.ID,
		Path:           `C:\removed`,
		Trustee:        `DOMAIN\removed`,
		TrusteeSID:     "S-1-5-21-100",
		ChangeType:     "removed",
		PreviousRights: "Read",
		DetectedAt:     changesByKey["removed|C:\\removed"].DetectedAt,
	}, changesByKey["removed|C:\\removed"])

	assert.Equal(t, models.PermissionChange{
		ID:             changesByKey["modified|C:\\modified"].ID,
		ScanSessionID:  currentSession.ID,
		Path:           `C:\modified`,
		Trustee:        `DOMAIN\modified`,
		TrusteeSID:     "S-1-5-21-200",
		ChangeType:     "modified",
		PreviousRights: "Read",
		CurrentRights:  "Full Control",
		DetectedAt:     changesByKey["modified|C:\\modified"].DetectedAt,
	}, changesByKey["modified|C:\\modified"])
}

func TestDetectChanges_UsesTrusteeSIDAsComparisonKey(t *testing.T) {
	baselineSession := &models.ScanSession{ID: uuid.New()}
	currentSession := &models.ScanSession{ID: uuid.New()}
	engine := NewComparisonEngine(baselineSession, currentSession)

	baselinePerms := []models.Permission{
		newPermission(`C:\shared`, "DOMAIN\\alice", "S-1-5-21-500", "Read"),
	}
	currentPerms := []models.Permission{
		newPermission(`C:\shared`, "alice@domain.local", "S-1-5-21-500", "Read"),
	}

	report, err := engine.DetectChanges(baselinePerms, currentPerms)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Empty(t, report.Changes)
	assert.Equal(t, 0, report.ChangesCount)
}

func newPermission(path, trustee, trusteeSID, rights string) models.Permission {
	return models.Permission{
		Path:       path,
		Trustee:    trustee,
		TrusteeSID: trusteeSID,
		Rights:     rights,
	}
}
