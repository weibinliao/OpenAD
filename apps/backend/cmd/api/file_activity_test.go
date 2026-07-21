package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWindowsFileActivityXML(t *testing.T) {
	raw := []byte(`<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">
  <System>
    <Provider Name="Microsoft-Windows-Security-Auditing" />
    <EventID>4663</EventID>
    <TimeCreated SystemTime="2026-05-08T10:00:00.0000000Z" />
    <Computer>FS01.internal.local</Computer>
  </System>
  <EventData>
    <Data Name="SubjectDomainName">DOMAIN</Data>
    <Data Name="SubjectUserSid">S-1-5-21-1000</Data>
    <Data Name="SubjectUserName">alice</Data>
    <Data Name="ObjectType">File</Data>
    <Data Name="ObjectName">C:\Shares\Finance\budget.xlsx</Data>
    <Data Name="AccessMask">0x2</Data>
    <Data Name="AccessList">WriteData</Data>
    <Data Name="ProcessName">C:\Windows\explorer.exe</Data>
  </EventData>
</Event>`)

	items, err := parseWindowsFileActivityXML(raw)
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.Equal(t, 4663, items[0].EventID)
	assert.Equal(t, "write", items[0].Action)
	assert.Equal(t, "DOMAIN\\alice", items[0].User)
	assert.Equal(t, "S-1-5-21-1000", items[0].UserSID)
	assert.Equal(t, "C:\\Shares\\Finance\\budget.xlsx", items[0].Path)
	assert.Equal(t, "FS01.internal.local", items[0].Computer)
}

func TestShareAccessEventBuildsPathFromShareData(t *testing.T) {
	raw := []byte(`<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">
  <System>
    <Provider Name="Microsoft-Windows-Security-Auditing" />
    <EventID>5145</EventID>
    <TimeCreated SystemTime="2026-05-08T10:05:00.0000000Z" />
    <Computer>FS01</Computer>
  </System>
  <EventData>
    <Data Name="SubjectDomainName">DOMAIN</Data>
    <Data Name="SubjectUserName">bob</Data>
    <Data Name="ShareName">\\*\Finance</Data>
    <Data Name="RelativeTargetName">Reports\q1.xlsx</Data>
    <Data Name="AccessMask">0x1</Data>
    <Data Name="IpAddress">192.0.2.25</Data>
  </EventData>
</Event>`)

	items, err := parseWindowsFileActivityXML(raw)
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.Equal(t, "read", items[0].Action)
	assert.Equal(t, "\\\\*\\Finance\\Reports\\q1.xlsx", items[0].Path)
	assert.Equal(t, "192.0.2.25", items[0].ClientIP)
}

func TestFileActivityEndpointReturnsProviderEvents(t *testing.T) {
	originalProvider := fileActivityEventProvider
	fileActivityEventProvider = func(_ context.Context, query fileActivityQuery) ([]fileActivityEvent, fileActivitySource, error) {
		return filterFileActivityEvents([]fileActivityEvent{
			{
				EventID:    4663,
				Action:     "write",
				User:       "DOMAIN\\alice",
				Path:       "C:\\Shares\\Finance\\budget.xlsx",
				AccessMask: "0x2",
				Computer:   "FS01",
				Timestamp:  time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
				Source:     "test",
			},
		}, query), buildFileActivitySource(query), nil
	}
	t.Cleanup(func() { fileActivityEventProvider = originalProvider })

	router := newTestRouter(applicationDependencies{})
	request := httptest.NewRequest(http.MethodGet, "/api/file-activity/events?limit=20&hours=48&path=finance&user=alice&action=write", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Items   []fileActivityEvent `json:"items"`
		Summary fileActivitySummary `json:"summary"`
		Source  fileActivitySource  `json:"source"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Items, 1)
	assert.Equal(t, 1, response.Summary.Write)
	assert.False(t, response.Source.ContentScanning)
	assert.Equal(t, 48, response.Source.Hours)
}

func TestFileActivityPostQueryResolvesSIDWithActiveDirectory(t *testing.T) {
	originalProvider := fileActivityEventProvider
	fileActivityEventProvider = func(_ context.Context, query fileActivityQuery) ([]fileActivityEvent, fileActivitySource, error) {
		return []fileActivityEvent{
			{
				EventID:    4663,
				Action:     "read",
				User:       "S-1-5-21-1000",
				RawUser:    "S-1-5-21-1000",
				UserSID:    "S-1-5-21-1000",
				Path:       "C:\\Shares\\Finance\\budget.xlsx",
				AccessMask: "0x1",
				Computer:   "FS01",
				Timestamp:  time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
				Source:     "test",
			},
		}, buildFileActivitySource(query), nil
	}
	t.Cleanup(func() { fileActivityEventProvider = originalProvider })

	adClient := &stubADGroupClient{principalsByDN: map[string]models.ADPrincipal{
		"s-1-5-21-1000": {
			DN:             "CN=Alice,OU=Users,DC=example,DC=com",
			Name:           "Alice Chen",
			SAMAccountName: "alice",
			SID:            "S-1-5-21-1000",
			Domain:         "EXAMPLE",
			Type:           models.ADObjectTypeUser,
		},
	}}
	router := newTestRouter(applicationDependencies{ad: &stubADService{groupClient: adClient}})

	recorder := performJSONRequestToRouter(t, router, "/api/file-activity/events/query", fileActivityQueryRequest{
		Limit: 20,
		Hours: 24,
		ADResolution: fileActivityADRequest{
			Enabled:  true,
			Server:   "ldap://directory.example.com",
			BaseDN:   "DC=example,DC=com",
			Username: "svc",
			Password: "secret",
		},
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Items  []fileActivityEvent `json:"items"`
		Source fileActivitySource  `json:"source"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Items, 1)
	assert.Equal(t, "EXAMPLE\\alice (Alice Chen)", response.Items[0].User)
	assert.Equal(t, "active-directory", response.Items[0].Resolution)
	assert.Equal(t, "S-1-5-21-1000", response.Items[0].UserSID)
	assert.Equal(t, "enabled", response.Source.ADResolution)
	assert.Equal(t, 1, response.Source.ADResolved)
}

func TestParseAuditPolicyStates(t *testing.T) {
	states := parseAuditPolicyStates(`Machine Name,Policy Target,Subcategory,Subcategory GUID,Inclusion Setting,Exclusion Setting
FS01,System,File System,{0CCE921D-69AE-11D9-BED3-505054503030},Success and Failure,
FS01,System,File Share,{0CCE9224-69AE-11D9-BED3-505054503030},No Auditing,
FS01,System,Detailed File Share,{0CCE9244-69AE-11D9-BED3-505054503030},Success,`)

	assert.True(t, states[normalizeAuditPolicyGUID("0CCE921D-69AE-11D9-BED3-505054503030")].Enabled)
	assert.False(t, states[normalizeAuditPolicyGUID("0CCE9224-69AE-11D9-BED3-505054503030")].Enabled)
	assert.True(t, states[normalizeAuditPolicyGUID("0CCE9244-69AE-11D9-BED3-505054503030")].Enabled)
}

func TestFileActivityReadinessEndpointReportsChecks(t *testing.T) {
	originalRunner := fileActivityCommandRunner
	originalOS := fileActivityRuntimeOS
	fileActivityRuntimeOS = "windows"
	fileActivityCommandRunner = func(_ context.Context, name string, _ ...string) ([]byte, error) {
		switch name {
		case "wevtutil":
			return []byte(`<Event><System><EventID>4663</EventID></System></Event>`), nil
		case "cmd":
			return []byte(`Machine Name,Policy Target,Subcategory,Subcategory GUID,Inclusion Setting,Exclusion Setting
FS01,System,File System,{0CCE921D-69AE-11D9-BED3-505054503030},Success and Failure,
FS01,System,File Share,{0CCE9224-69AE-11D9-BED3-505054503030},No Auditing,
FS01,System,Detailed File Share,{0CCE9244-69AE-11D9-BED3-505054503030},Success,`), nil
		case "powershell":
			return []byte("SACL_PRESENT"), nil
		default:
			return []byte{}, nil
		}
	}
	t.Cleanup(func() {
		fileActivityCommandRunner = originalRunner
		fileActivityRuntimeOS = originalOS
	})

	router := newTestRouter(applicationDependencies{})
	request := httptest.NewRequest(http.MethodGet, "/api/file-activity/readiness?path=C%3A%5CShares%5CFinance", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response fileActivityReadinessResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "warning", response.Status)
	assert.Equal(t, "windows", response.HostOS)
	assert.NotEmpty(t, response.Commands)
	assert.Contains(t, response.TargetPath, `C:\Shares\Finance`)
	assert.Contains(t, readinessCheckStatuses(response.Checks), "target_sacl:ok")
	assert.Contains(t, readinessCheckStatuses(response.Checks), "audit_policy_file_share:warning")
}

func TestFileActivityEndpointRejectsInvalidAction(t *testing.T) {
	router := newTestRouter(applicationDependencies{})
	request := httptest.NewRequest(http.MethodGet, "/api/file-activity/events?action=content", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func readinessCheckStatuses(checks []fileActivityReadinessCheck) []string {
	result := make([]string, 0, len(checks))
	for _, check := range checks {
		result = append(result, check.ID+":"+check.Status)
	}
	return result
}
