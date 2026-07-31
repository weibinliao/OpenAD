package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/riskservice"
)

type stubRiskFindingStore struct {
	items          []models.RiskFinding
	listErr        error
	upsertInputs   []riskservice.FindingInput
	upsertErr      error
	importInputs   []riskservice.FindingInput
	importErr      error
	updatedID      string
	updatedStatus  string
	updatedNote    *string
	updatedFinding *models.RiskFinding
	updateErr      error
}

func (store *stubRiskFindingStore) List() ([]models.RiskFinding, error) {
	return store.items, store.listErr
}

func (store *stubRiskFindingStore) UpsertFromScan(inputs []riskservice.FindingInput) (int, error) {
	store.upsertInputs = inputs
	return len(inputs), store.upsertErr
}

func (store *stubRiskFindingStore) ImportLegacy(inputs []riskservice.FindingInput) (int, error) {
	store.importInputs = inputs
	return len(inputs), store.importErr
}

func (store *stubRiskFindingStore) UpdateStatus(id, status string, note *string) (*models.RiskFinding, error) {
	store.updatedID = id
	store.updatedStatus = status
	store.updatedNote = note
	return store.updatedFinding, store.updateErr
}

func riskFindingTestRouter(store riskFindingStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	application := newApplication(applicationDependencies{riskFindings: store})
	router := gin.New()
	router.GET("/api/risk-findings", application.handleListRiskFindings)
	router.POST("/api/risk-findings/upsert", application.handleUpsertRiskFindings)
	router.POST("/api/risk-findings/import", application.handleImportRiskFindings)
	router.PUT("/api/risk-findings/:id", application.handleUpdateRiskFinding)
	return router
}

func riskFindingJSONRequest(t *testing.T, router *gin.Engine, method, target string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestRiskFindingEndpointsPersistAndReturnFindings(t *testing.T) {
	id := uuid.New()
	scannedAt := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	store := &stubRiskFindingStore{items: []models.RiskFinding{{
		ID: id, Fingerprint: "fp-1", Status: "open", Severity: "high",
		Type: "broad-access", Title: "Broad access", Path: `C:\Data`,
		Trustee: "Everyone", FirstSeenAt: scannedAt, LastSeenAt: scannedAt, SeenCount: 1,
	}}}
	router := riskFindingTestRouter(store)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/risk-findings", nil)
	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, listRequest)
	require.Equal(t, http.StatusOK, listRecorder.Code)
	assert.Contains(t, listRecorder.Body.String(), `"count":1`)
	assert.Contains(t, listRecorder.Body.String(), `"fingerprint":"fp-1"`)

	payload := map[string]any{"items": []map[string]any{{
		"fingerprint": "fp-2", "path": `C:\Finance`, "trustee": "Everyone",
		"last_seen_at": scannedAt.Format(time.RFC3339),
	}}}
	upsertRecorder := riskFindingJSONRequest(t, router, http.MethodPost, "/api/risk-findings/upsert", payload)
	require.Equal(t, http.StatusOK, upsertRecorder.Code)
	require.Len(t, store.upsertInputs, 1)
	assert.Equal(t, "fp-2", store.upsertInputs[0].Fingerprint)

	importRecorder := riskFindingJSONRequest(t, router, http.MethodPost, "/api/risk-findings/import", payload)
	require.Equal(t, http.StatusOK, importRecorder.Code)
	require.Len(t, store.importInputs, 1)

	note := "Approved exception"
	store.updatedFinding = &models.RiskFinding{ID: id, Fingerprint: "fp-1", Status: "accepted"}
	updateRecorder := riskFindingJSONRequest(t, router, http.MethodPut, "/api/risk-findings/"+id.String(), map[string]any{
		"status": "accepted",
		"note":   note,
	})
	require.Equal(t, http.StatusOK, updateRecorder.Code)
	assert.Equal(t, id.String(), store.updatedID)
	assert.Equal(t, "accepted", store.updatedStatus)
	require.NotNil(t, store.updatedNote)
	assert.Equal(t, note, *store.updatedNote)
}

func TestRiskFindingEndpointsReportDatabaseUnavailable(t *testing.T) {
	store := &stubRiskFindingStore{listErr: riskservice.ErrDatabaseUnavailable}
	router := riskFindingTestRouter(store)

	request := httptest.NewRequest(http.MethodGet, "/api/risk-findings", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "database is not initialized")
}

func TestRiskFindingEndpointsValidateRequests(t *testing.T) {
	store := &stubRiskFindingStore{updateErr: errors.New("unexpected update")}
	router := riskFindingTestRouter(store)

	badJSON := httptest.NewRequest(http.MethodPost, "/api/risk-findings/upsert", bytes.NewBufferString("{"))
	badJSON.Header.Set("Content-Type", "application/json")
	badJSONRecorder := httptest.NewRecorder()
	router.ServeHTTP(badJSONRecorder, badJSON)
	assert.Equal(t, http.StatusBadRequest, badJSONRecorder.Code)

	badStatus := riskFindingJSONRequest(t, router, http.MethodPut, "/api/risk-findings/"+uuid.NewString(), map[string]any{})
	assert.Equal(t, http.StatusBadRequest, badStatus.Code)
}
