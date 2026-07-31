package riskservice

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weibinliao/OpenAD/internal/models"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) *Service {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RiskFinding{}))
	require.NoError(t, db.Exec("DELETE FROM risk_findings").Error)

	return New(db)
}

func testFinding(scannedAt time.Time) FindingInput {
	return FindingInput{
		Fingerprint:       "broad-access|c:\\finance|everyone|modify",
		Severity:          "critical",
		Type:              "broad-access",
		Title:             "Broad access to Finance",
		SuggestedAction:   "Remove broad write access.",
		Path:              `C:\Finance`,
		Trustee:           "Everyone",
		TrusteeSID:        "S-1-1-0",
		Rights:            "Modify",
		Source:            "Explicit",
		FirstSeenAt:       scannedAt,
		LastSeenAt:        scannedAt,
		LastSessionID:     "session-1",
		SeenCount:         1,
		Category:          "sensitive-data",
		PriorityScore:     96,
		Confidence:        "high",
		RemediationEffort: "quick-win",
		ControlMapping:    []string{"AC-3"},
		Evidence:          []string{"Everyone has Modify"},
		SensitiveLabels:   []string{"Finance"},
	}
}

func TestUpsertFromScanMergesByFingerprintAndPreservesReview(t *testing.T) {
	service := newTestService(t)
	firstSeen := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)

	count, err := service.UpsertFromScan([]FindingInput{testFinding(firstSeen)})
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	items, err := service.List()
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "open", items[0].Status)
	assert.Equal(t, 1, items[0].SeenCount)

	note := "Approved exception CR-42"
	updated, err := service.UpdateStatus(items[0].ID.String(), "accepted", &note)
	require.NoError(t, err)
	assert.Equal(t, "accepted", updated.Status)
	assert.Equal(t, note, updated.Note)

	secondSeen := firstSeen.Add(24 * time.Hour)
	second := testFinding(secondSeen)
	second.Title = "Updated broad access title"
	second.LastSessionID = "session-2"

	count, err = service.UpsertFromScan([]FindingInput{second})
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	items, err = service.List()
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "accepted", items[0].Status)
	assert.Equal(t, note, items[0].Note)
	assert.Equal(t, 2, items[0].SeenCount)
	assert.Equal(t, firstSeen, items[0].FirstSeenAt)
	assert.Equal(t, secondSeen, items[0].LastSeenAt)
	assert.Equal(t, "session-2", items[0].LastSessionID)
	assert.Equal(t, "Updated broad access title", items[0].Title)

	_, err = service.UpdateStatus(items[0].ID.String(), "resolved", nil)
	require.NoError(t, err)
	_, err = service.UpsertFromScan([]FindingInput{testFinding(secondSeen.Add(time.Hour))})
	require.NoError(t, err)

	items, err = service.List()
	require.NoError(t, err)
	assert.Equal(t, "open", items[0].Status)
}

func TestImportLegacyIsIdempotent(t *testing.T) {
	service := newTestService(t)
	scannedAt := time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC)
	legacy := testFinding(scannedAt)
	legacy.Status = "accepted"
	legacy.Note = "Legacy review note"
	legacy.SeenCount = 5

	for range 2 {
		count, err := service.ImportLegacy([]FindingInput{legacy})
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	}

	items, err := service.List()
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "accepted", items[0].Status)
	assert.Equal(t, "Legacy review note", items[0].Note)
	assert.Equal(t, 5, items[0].SeenCount)
	assert.Equal(t, scannedAt, items[0].FirstSeenAt)
	assert.Equal(t, scannedAt, items[0].LastSeenAt)
}

func TestUpdateStatusValidatesInput(t *testing.T) {
	service := newTestService(t)
	_, err := service.UpsertFromScan([]FindingInput{testFinding(time.Now().UTC())})
	require.NoError(t, err)

	items, err := service.List()
	require.NoError(t, err)

	_, err = service.UpdateStatus(items[0].ID.String(), "ignored", nil)
	assert.ErrorIs(t, err, ErrInvalidStatus)

	_, err = service.UpdateStatus("not-a-uuid", "accepted", nil)
	assert.ErrorIs(t, err, ErrInvalidID)
}
