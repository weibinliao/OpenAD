package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/weibinliao/OpenAD/internal/database"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// withAccessTestDatabase swaps the global database handle for an in-memory
// sqlite instance for the duration of the test.
func withAccessTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.ScanSession{},
		&models.Permission{},
		&models.DirectorySyncRun{},
		&models.ADUserRecord{},
		&models.ADGroupRecord{},
		&models.ADMembershipRecord{},
	))

	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous })

	return db
}

func seedAccessFixture(t *testing.T, db *gorm.DB) {
	t.Helper()

	now := time.Now().UTC()
	finished := now.Add(time.Minute)

	run := models.DirectorySyncRun{Status: "completed", StartedAt: now, FinishedAt: &finished}
	require.NoError(t, db.Create(&run).Error)
	require.NoError(t, db.Create(&models.ADUserRecord{
		RunID: run.ID, SID: "S-1-5-21-9-9-9-1001", SAMAccountName: "alice", DisplayName: "Alice", Enabled: true,
	}).Error)
	require.NoError(t, db.Create(&models.ADGroupRecord{
		RunID: run.ID, SID: "S-1-5-21-9-9-9-2001", Name: "Sales",
	}).Error)
	require.NoError(t, db.Create(&models.ADMembershipRecord{
		RunID: run.ID, MemberSID: "S-1-5-21-9-9-9-1001", GroupSID: "S-1-5-21-9-9-9-2001", Direct: true,
	}).Error)

	session := models.ScanSession{RootPath: `D:\Share`, Status: "completed", StartedAt: now, FinishedAt: &finished}
	require.NoError(t, db.Create(&session).Error)
	require.NoError(t, db.Create(&[]models.Permission{
		{ScanSessionID: session.ID, Path: `D:\Share\Sales`, Trustee: `CORP\Sales`, TrusteeSID: "S-1-5-21-9-9-9-2001", Rights: "Modify", Type: "allow"},
		{ScanSessionID: session.ID, Path: `D:\Share\Public`, Trustee: "Everyone", TrusteeSID: "S-1-1-0", Rights: "Read", Type: "allow"},
	}).Error)
}

func postJSON(t *testing.T, router http.Handler, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestAccessEndpointsUnavailableWithoutDatabase(t *testing.T) {
	previous := database.DB
	database.DB = nil
	t.Cleanup(func() { database.DB = previous })

	router := newTestRouter(applicationDependencies{})

	recorder := postJSON(t, router, "/api/access/by-user", jsonBody{"principal": "alice"})
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	recorder = postJSON(t, router, "/api/access/by-resource", jsonBody{"path_prefix": `D:\Share`})
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

// jsonBody is a shorthand for JSON request payload literals.
type jsonBody = map[string]any

func TestAccessByUserEndpoint(t *testing.T) {
	db := withAccessTestDatabase(t)
	router := newTestRouter(applicationDependencies{})

	// No completed sync run yet -> actionable 409.
	recorder := postJSON(t, router, "/api/access/by-user", jsonBody{"principal": "alice"})
	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "directory sync")

	seedAccessFixture(t, db)

	// Missing principal -> 400 from binding.
	recorder = postJSON(t, router, "/api/access/by-user", jsonBody{})
	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	// Unknown principal -> 404.
	recorder = postJSON(t, router, "/api/access/by-user", jsonBody{"principal": "nobody"})
	assert.Equal(t, http.StatusNotFound, recorder.Code)

	// Invalid sync_run_id -> 400.
	recorder = postJSON(t, router, "/api/access/by-user", jsonBody{"principal": "alice", "sync_run_id": "not-a-uuid"})
	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	// Happy path: alice reaches D:\Share\Sales via the Sales group.
	recorder = postJSON(t, router, "/api/access/by-user", jsonBody{"principal": "alice"})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var response struct {
		User struct {
			SID string `json:"sid"`
		} `json:"user"`
		GroupCount int `json:"group_count"`
		Entries    []struct {
			Path string `json:"path"`
			Why  struct {
				Kind      string `json:"kind"`
				GroupName string `json:"group_name"`
			} `json:"why"`
		} `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "S-1-5-21-9-9-9-1001", response.User.SID)
	assert.Equal(t, 1, response.GroupCount)
	require.Len(t, response.Entries, 1)
	assert.Equal(t, `D:\Share\Sales`, response.Entries[0].Path)
	assert.Equal(t, "group", response.Entries[0].Why.Kind)
	assert.Equal(t, "Sales", response.Entries[0].Why.GroupName)
}

func TestAccessByResourceEndpoint(t *testing.T) {
	db := withAccessTestDatabase(t)
	router := newTestRouter(applicationDependencies{})
	seedAccessFixture(t, db)

	// Missing path_prefix -> 400 from binding.
	recorder := postJSON(t, router, "/api/access/by-resource", jsonBody{})
	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	// Path no session covers -> actionable 409.
	recorder = postJSON(t, router, "/api/access/by-resource", jsonBody{"path_prefix": `Z:\Nowhere`})
	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "scan")

	// Happy path: Sales expands to alice, Everyone stays as unresolved.
	recorder = postJSON(t, router, "/api/access/by-resource", jsonBody{"path_prefix": `D:\Share`})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var response struct {
		Principals []struct {
			SID    string `json:"sid"`
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"principals"`
		ACEs []struct {
			TrusteeSID string `json:"trustee_sid"`
		} `json:"aces"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.ACEs, 2)

	sources := map[string]string{}
	names := map[string]string{}
	for _, principal := range response.Principals {
		sources[principal.SID] = principal.Source
		names[principal.SID] = principal.Name
	}
	assert.Equal(t, "group-member", sources["S-1-5-21-9-9-9-1001"])
	assert.Equal(t, "unresolved", sources["S-1-1-0"])
	assert.Equal(t, "Everyone", names["S-1-1-0"])
}
