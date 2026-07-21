package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	userpkg "os/user"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/gin-gonic/gin"
)

const (
	fileActivityDefaultLimit = 100
	fileActivityMaxLimit     = 500
	fileActivityDefaultHours = 24
	fileActivityMaxHours     = 168
)

var fileActivityEventProvider = queryWindowsSecurityFileActivityEvents
var fileActivityCommandRunner = runFileActivityCommand
var fileActivityRuntimeOS = runtime.GOOS

var requiredFileActivityAuditPolicies = []auditPolicyState{
	{Name: "File System", GUID: "0CCE921D-69AE-11D9-BED3-505054503030"},
	{Name: "File Share", GUID: "0CCE9224-69AE-11D9-BED3-505054503030"},
	{Name: "Detailed File Share", GUID: "0CCE9244-69AE-11D9-BED3-505054503030"},
}

type fileActivityQuery struct {
	Limit  int
	Hours  int
	Path   string
	User   string
	Action string
}

type fileActivityQueryRequest struct {
	Limit        int                   `json:"limit"`
	Hours        int                   `json:"hours"`
	Path         string                `json:"path"`
	User         string                `json:"user"`
	Action       string                `json:"action"`
	ADResolution fileActivityADRequest `json:"ad_resolution"`
}

type fileActivityADRequest struct {
	Enabled  bool   `json:"enabled"`
	Server   string `json:"server"`
	BaseDN   string `json:"base_dn"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type fileActivityEvent struct {
	EventID     int       `json:"event_id"`
	Action      string    `json:"action"`
	User        string    `json:"user"`
	RawUser     string    `json:"raw_user,omitempty"`
	UserSID     string    `json:"user_sid,omitempty"`
	UserDisplay string    `json:"user_display,omitempty"`
	UserDN      string    `json:"user_dn,omitempty"`
	UserType    string    `json:"user_type,omitempty"`
	Resolution  string    `json:"resolution"`
	Domain      string    `json:"domain"`
	Path        string    `json:"path"`
	ObjectType  string    `json:"object_type"`
	AccessMask  string    `json:"access_mask"`
	AccessList  string    `json:"access_list"`
	ProcessName string    `json:"process_name"`
	ClientIP    string    `json:"client_ip,omitempty"`
	Computer    string    `json:"computer"`
	Timestamp   time.Time `json:"timestamp"`
	Source      string    `json:"source"`
}

type fileActivitySource struct {
	Provider        string `json:"provider"`
	Scope           string `json:"scope"`
	Hours           int    `json:"hours"`
	Limit           int    `json:"limit"`
	ContentScanning bool   `json:"content_scanning"`
	Requires        string `json:"requires"`
	RemoteNote      string `json:"remote_note"`
	ADResolution    string `json:"ad_resolution"`
	ADResolved      int    `json:"ad_resolved"`
	ADUnresolved    int    `json:"ad_unresolved"`
	ADWarning       string `json:"ad_warning,omitempty"`
}

type fileActivitySummary struct {
	Total             int `json:"total"`
	Read              int `json:"read"`
	Write             int `json:"write"`
	Delete            int `json:"delete"`
	PermissionChanges int `json:"permission_changes"`
	ShareAccess       int `json:"share_access"`
	Other             int `json:"other"`
}

type fileActivityReadinessResponse struct {
	Status      string                       `json:"status"`
	HostOS      string                       `json:"host_os"`
	CurrentUser string                       `json:"current_user"`
	TargetPath  string                       `json:"target_path,omitempty"`
	GeneratedAt time.Time                    `json:"generated_at"`
	Checks      []fileActivityReadinessCheck `json:"checks"`
	Commands    []fileActivitySetupCommand   `json:"commands"`
	NextSteps   []string                     `json:"next_steps"`
}

type fileActivityReadinessCheck struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	Detail      string `json:"detail"`
	Remediation string `json:"remediation,omitempty"`
}

type fileActivitySetupCommand struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command"`
	Requires    string `json:"requires"`
}

type auditPolicyState struct {
	Name    string
	GUID    string
	Setting string
	Enabled bool
	Found   bool
}

type windowsEventsXML struct {
	Events []windowsEventXML `xml:"Event"`
}

type windowsEventXML struct {
	System struct {
		Provider struct {
			Name string `xml:"Name,attr"`
		} `xml:"Provider"`
		EventID     int    `xml:"EventID"`
		Computer    string `xml:"Computer"`
		TimeCreated struct {
			SystemTime string `xml:"SystemTime,attr"`
		} `xml:"TimeCreated"`
	} `xml:"System"`
	EventData struct {
		Data []struct {
			Name  string `xml:"Name,attr"`
			Value string `xml:",chardata"`
		} `xml:"Data"`
	} `xml:"EventData"`
}

func (application *application) handleListFileActivityEvents(context *gin.Context) {
	query, err := parseFileActivityQuery(context)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	items, source, err := fileActivityEventProvider(context.Request.Context(), query)
	if err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, errFileActivityUnsupported) {
			status = http.StatusNotImplemented
		}
		context.JSON(status, gin.H{
			"error":  err.Error(),
			"source": source,
		})
		return
	}

	context.JSON(http.StatusOK, gin.H{
		"items":   items,
		"summary": summarizeFileActivityEvents(items),
		"source":  source,
	})
}

func (application *application) handleQueryFileActivityEvents(context *gin.Context) {
	var request fileActivityQueryRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query, err := fileActivityQueryFromRequest(request)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	items, source, err := fileActivityEventProvider(context.Request.Context(), query)
	if err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, errFileActivityUnsupported) {
			status = http.StatusNotImplemented
		}
		context.JSON(status, gin.H{
			"error":  err.Error(),
			"source": source,
		})
		return
	}

	items, source = application.resolveFileActivityADIdentities(context.Request.Context(), items, source, request.ADResolution)
	context.JSON(http.StatusOK, gin.H{
		"items":   items,
		"summary": summarizeFileActivityEvents(items),
		"source":  source,
	})
}

func (application *application) handleFileActivityReadiness(context *gin.Context) {
	targetPath := strings.TrimSpace(context.Query("path"))
	readiness := buildFileActivityReadiness(context.Request.Context(), targetPath)
	context.JSON(http.StatusOK, readiness)
}

var errFileActivityUnsupported = errors.New("file activity audit is only available on Windows hosts")

func runFileActivityCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	return command.CombinedOutput()
}

func buildFileActivityReadiness(parent context.Context, targetPath string) fileActivityReadinessResponse {
	checks := []fileActivityReadinessCheck{
		buildFileActivityOSCheck(),
	}
	if fileActivityRuntimeOS == "windows" {
		checks = append(checks, buildSecurityLogReadinessCheck(parent))
		checks = append(checks, buildAuditPolicyReadinessChecks(parent)...)
		checks = append(checks, buildTargetSACLReadinessCheck(parent, targetPath))
	}

	return fileActivityReadinessResponse{
		Status:      summarizeReadinessStatus(checks),
		HostOS:      fileActivityRuntimeOS,
		CurrentUser: currentFileActivityUser(),
		TargetPath:  strings.TrimSpace(targetPath),
		GeneratedAt: time.Now().UTC(),
		Checks:      checks,
		Commands:    buildFileActivitySetupCommands(targetPath),
		NextSteps:   buildFileActivityNextSteps(targetPath),
	}
}

func buildFileActivityOSCheck() fileActivityReadinessCheck {
	if fileActivityRuntimeOS != "windows" {
		return fileActivityReadinessCheck{
			ID:          "windows_host",
			Label:       "Windows file server host",
			Status:      "fail",
			Detail:      "File activity audit requires Windows Security event logs.",
			Remediation: "Run OpenAD on the Windows file server, or forward Security events to a Windows host running the API.",
		}
	}

	return fileActivityReadinessCheck{
		ID:     "windows_host",
		Label:  "Windows file server host",
		Status: "ok",
		Detail: "The API is running on Windows, so Windows Security event logs can be queried.",
	}
}

func buildSecurityLogReadinessCheck(parent context.Context) fileActivityReadinessCheck {
	ctx, cancel := context.WithTimeout(parent, 6*time.Second)
	defer cancel()

	output, err := fileActivityCommandRunner(ctx, "wevtutil", "qe", "Security", "/c:1", "/f:xml", "/rd:true")
	if ctx.Err() == context.DeadlineExceeded {
		return fileActivityReadinessCheck{
			ID:          "security_log_read",
			Label:       "Security log read permission",
			Status:      "fail",
			Detail:      "Security log query timed out.",
			Remediation: "Run the API as Administrator or an account in Event Log Readers, then try readiness again.",
		}
	}
	if err != nil {
		message := sanitizeWindowsEventLogError(strings.TrimSpace(decodeWindowsCommandOutput(output)))
		lower := strings.ToLower(message)
		if strings.Contains(lower, "no events") {
			return fileActivityReadinessCheck{
				ID:          "security_log_read",
				Label:       "Security log read permission",
				Status:      "warning",
				Detail:      "Security log was reachable, but no sample event was returned.",
				Remediation: "Generate a file access event after enabling Object Access auditing and SACLs.",
			}
		}
		return fileActivityReadinessCheck{
			ID:          "security_log_read",
			Label:       "Security log read permission",
			Status:      "fail",
			Detail:      message,
			Remediation: "Run the API as Administrator or an account in Event Log Readers.",
		}
	}

	return fileActivityReadinessCheck{
		ID:     "security_log_read",
		Label:  "Security log read permission",
		Status: "ok",
		Detail: "The API can read sample metadata from the Windows Security log.",
	}
}

func buildAuditPolicyReadinessChecks(parent context.Context) []fileActivityReadinessCheck {
	ctx, cancel := context.WithTimeout(parent, 6*time.Second)
	defer cancel()

	output, err := fileActivityCommandRunner(ctx, "cmd", "/c", "chcp 65001 >NUL & auditpol /get /category:* /r")
	if ctx.Err() == context.DeadlineExceeded {
		return []fileActivityReadinessCheck{{
			ID:          "audit_policy_query",
			Label:       "Windows audit policy verification",
			Status:      "fail",
			Detail:      "auditpol query timed out.",
			Remediation: "Run the API with local administrator rights, or verify Object Access audit policy manually.",
		}}
	}
	if err != nil {
		message := strings.TrimSpace(decodeWindowsCommandOutput(output))
		if message == "" {
			message = err.Error()
		}
		return []fileActivityReadinessCheck{{
			ID:          "audit_policy_query",
			Label:       "Windows audit policy verification",
			Status:      "fail",
			Detail:      sanitizeWindowsEventLogError(message),
			Remediation: "Run OpenAD as Administrator to verify or change Object Access audit policy.",
		}}
	}

	states := parseAuditPolicyStates(decodeWindowsCommandOutput(output))
	checks := make([]fileActivityReadinessCheck, 0, len(requiredFileActivityAuditPolicies))
	for _, required := range requiredFileActivityAuditPolicies {
		state := states[normalizeAuditPolicyGUID(required.GUID)]
		if !state.Found {
			checks = append(checks, fileActivityReadinessCheck{
				ID:          "audit_policy_" + strings.ToLower(strings.ReplaceAll(required.Name, " ", "_")),
				Label:       required.Name + " audit policy",
				Status:      "warning",
				Detail:      "auditpol output did not include this subcategory.",
				Remediation: "Verify the Object Access audit policy manually on the file server.",
			})
			continue
		}
		if state.Enabled {
			checks = append(checks, fileActivityReadinessCheck{
				ID:     "audit_policy_" + strings.ToLower(strings.ReplaceAll(required.Name, " ", "_")),
				Label:  required.Name + " audit policy",
				Status: "ok",
				Detail: fmt.Sprintf("Current setting: %s.", state.Setting),
			})
			continue
		}
		checks = append(checks, fileActivityReadinessCheck{
			ID:          "audit_policy_" + strings.ToLower(strings.ReplaceAll(required.Name, " ", "_")),
			Label:       required.Name + " audit policy",
			Status:      "warning",
			Detail:      fmt.Sprintf("Current setting: %s.", firstNonEmpty(state.Setting, "not enabled")),
			Remediation: "Enable Success auditing for this subcategory before expecting file activity events.",
		})
	}
	return checks
}

func buildTargetSACLReadinessCheck(parent context.Context, targetPath string) fileActivityReadinessCheck {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return fileActivityReadinessCheck{
			ID:          "target_sacl",
			Label:       "Target folder SACL",
			Status:      "info",
			Detail:      "Enter a target folder or share path to verify whether audit entries are present.",
			Remediation: "Use a small pilot folder first, then expand after validating event volume.",
		}
	}

	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	script := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$path = '%s'
if (-not (Test-Path -LiteralPath $path)) { Write-Output 'PATH_MISSING'; exit 2 }
$acl = Get-Acl -LiteralPath $path -Audit
if ($acl.Audit.Count -gt 0) { Write-Output 'SACL_PRESENT' } else { Write-Output 'SACL_EMPTY' }`, escapePowerShellSingleQuoted(targetPath))
	output, err := fileActivityCommandRunner(ctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodePowerShellCommand(script))
	message := strings.TrimSpace(decodeWindowsCommandOutput(output))
	if ctx.Err() == context.DeadlineExceeded {
		return fileActivityReadinessCheck{
			ID:          "target_sacl",
			Label:       "Target folder SACL",
			Status:      "warning",
			Detail:      "SACL verification timed out for the target path.",
			Remediation: "Verify the target path locally on the file server with Get-Acl -Audit.",
		}
	}
	if strings.Contains(message, "SACL_PRESENT") {
		return fileActivityReadinessCheck{
			ID:     "target_sacl",
			Label:  "Target folder SACL",
			Status: "ok",
			Detail: "The target path has at least one audit entry.",
		}
	}
	if strings.Contains(message, "SACL_EMPTY") {
		return fileActivityReadinessCheck{
			ID:          "target_sacl",
			Label:       "Target folder SACL",
			Status:      "warning",
			Detail:      "The target path exists, but no audit entries were found.",
			Remediation: "Add a folder audit rule for the users or groups whose access should be recorded.",
		}
	}
	if strings.Contains(message, "PATH_MISSING") {
		return fileActivityReadinessCheck{
			ID:          "target_sacl",
			Label:       "Target folder SACL",
			Status:      "warning",
			Detail:      "The target path was not found from this host.",
			Remediation: "Run the check on the file server or enter a reachable local/UNC path.",
		}
	}
	if err != nil {
		if message == "" {
			message = err.Error()
		}
		return fileActivityReadinessCheck{
			ID:          "target_sacl",
			Label:       "Target folder SACL",
			Status:      "warning",
			Detail:      sanitizeWindowsEventLogError(message),
			Remediation: "Run as Administrator or verify SACLs manually with Get-Acl -Audit.",
		}
	}

	return fileActivityReadinessCheck{
		ID:          "target_sacl",
		Label:       "Target folder SACL",
		Status:      "info",
		Detail:      "SACL verification finished without a recognizable result.",
		Remediation: "Verify the target path manually with Get-Acl -Audit.",
	}
}

func currentFileActivityUser() string {
	current, err := userpkg.Current()
	if err != nil || current == nil {
		return "unknown"
	}
	return firstNonEmpty(current.Username, current.Name, current.Uid, "unknown")
}

func summarizeReadinessStatus(checks []fileActivityReadinessCheck) string {
	status := "ok"
	for _, check := range checks {
		switch check.Status {
		case "fail":
			return "fail"
		case "warning":
			status = "warning"
		}
	}
	return status
}

func buildFileActivitySetupCommands(targetPath string) []fileActivitySetupCommand {
	path := strings.TrimSpace(targetPath)
	if path == "" {
		path = `C:\Shares\Finance`
	}

	return []fileActivitySetupCommand{
		{
			ID:          "enable_audit_policy",
			Title:       "Enable Windows Object Access auditing",
			Description: "Run in elevated Command Prompt on the file server.",
			Command: strings.Join([]string{
				`auditpol /set /subcategory:"File System" /success:enable /failure:enable`,
				`auditpol /set /subcategory:"File Share" /success:enable /failure:enable`,
				`auditpol /set /subcategory:"Detailed File Share" /success:enable /failure:enable`,
			}, "\n"),
			Requires: "Local administrator",
		},
		{
			ID:          "add_folder_sacl",
			Title:       "Add a pilot folder audit rule",
			Description: "Run in elevated PowerShell. Start with a pilot folder to avoid excessive event volume.",
			Command: fmt.Sprintf(`$path = "%s"
$identity = "Domain Users"
$acl = Get-Acl -LiteralPath $path
$rule = New-Object System.Security.AccessControl.FileSystemAuditRule($identity, "ReadData,WriteData,AppendData,Delete,ChangePermissions,TakeOwnership", "ContainerInherit,ObjectInherit", "None", "Success,Failure")
$acl.AddAuditRule($rule)
Set-Acl -LiteralPath $path -AclObject $acl`, escapePowerShellDoubleQuoted(path)),
			Requires: "Local administrator or permission to edit SACLs",
		},
		{
			ID:          "generate_test_event",
			Title:       "Generate a test access event",
			Description: "After enabling policy and SACLs, access a test file and refresh File Activity.",
			Command:     fmt.Sprintf(`Get-ChildItem -LiteralPath "%s" | Select-Object -First 1`, escapePowerShellDoubleQuoted(path)),
			Requires:    "A domain user covered by the SACL rule",
		},
	}
}

func buildFileActivityNextSteps(targetPath string) []string {
	steps := []string{
		"Run OpenAD on the file server or forward Security events to this host.",
		"Connect AD in the AD Workspace before refreshing File Activity so SIDs resolve to domain names.",
		"Enable Object Access audit policy and add SACLs to a small pilot folder first.",
		"Generate one read/write test event, then refresh the access ledger.",
	}
	if strings.TrimSpace(targetPath) == "" {
		steps = append(steps, "Enter a target path in the readiness check to validate SACL presence.")
	}
	return steps
}

func parseAuditPolicyStates(raw string) map[string]auditPolicyState {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(strings.TrimSpace(raw), "\ufeff")))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil || len(rows) == 0 {
		return map[string]auditPolicyState{}
	}

	header := rows[0]
	guidIndex := indexOfAuditPolicyHeader(header, "guid")
	nameIndex := indexOfAuditPolicyHeader(header, "subcategory")
	settingIndex := indexOfAuditPolicyHeader(header, "inclusion")
	if settingIndex < 0 {
		settingIndex = indexOfAuditPolicyHeader(header, "setting")
	}

	states := make(map[string]auditPolicyState)
	for _, row := range rows[1:] {
		if len(row) == 0 {
			continue
		}
		rowGUIDIndex := guidIndex
		if rowGUIDIndex < 0 || rowGUIDIndex >= len(row) || normalizeAuditPolicyGUID(row[rowGUIDIndex]) == "" {
			rowGUIDIndex = findAuditPolicyGUIDIndex(row)
		}
		if rowGUIDIndex < 0 || rowGUIDIndex >= len(row) {
			continue
		}

		guid := normalizeAuditPolicyGUID(row[rowGUIDIndex])
		if guid == "" {
			continue
		}

		name := ""
		if nameIndex >= 0 && nameIndex < len(row) {
			name = strings.TrimSpace(row[nameIndex])
		} else if rowGUIDIndex > 0 {
			name = strings.TrimSpace(row[rowGUIDIndex-1])
		}

		setting := ""
		if settingIndex >= 0 && settingIndex < len(row) {
			setting = strings.TrimSpace(row[settingIndex])
		} else if rowGUIDIndex+1 < len(row) {
			setting = strings.TrimSpace(row[rowGUIDIndex+1])
		}

		states[guid] = auditPolicyState{
			Name:    name,
			GUID:    guid,
			Setting: setting,
			Enabled: auditPolicySettingEnabled(setting),
			Found:   true,
		}
	}
	return states
}

func indexOfAuditPolicyHeader(header []string, token string) int {
	for index, value := range header {
		if strings.Contains(strings.ToLower(strings.TrimSpace(value)), token) {
			return index
		}
	}
	return -1
}

func findAuditPolicyGUIDIndex(row []string) int {
	for index, value := range row {
		if normalizeAuditPolicyGUID(value) != "" {
			return index
		}
	}
	return -1
}

func normalizeAuditPolicyGUID(value string) string {
	trimmed := strings.ToUpper(strings.Trim(strings.TrimSpace(value), "{}"))
	if !strings.Contains(trimmed, "0CCE") || !strings.Contains(trimmed, "-") {
		return ""
	}
	return trimmed
}

func auditPolicySettingEnabled(setting string) bool {
	trimmed := strings.TrimSpace(setting)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	for _, token := range []string{"no auditing", "not configured", "none", "无审核", "未配置"} {
		if strings.Contains(lower, token) {
			return false
		}
	}
	return strings.Contains(lower, "success") || strings.Contains(lower, "failure") || strings.Contains(trimmed, "成功") || strings.Contains(trimmed, "失败")
}

func escapePowerShellSingleQuoted(value string) string {
	return strings.ReplaceAll(value, `'`, `''`)
}

func escapePowerShellDoubleQuoted(value string) string {
	return strings.ReplaceAll(value, `"`, "`\"")
}

func encodePowerShellCommand(script string) string {
	units := utf16.Encode([]rune(script))
	bytes := make([]byte, 0, len(units)*2)
	for _, unit := range units {
		bytes = append(bytes, byte(unit), byte(unit>>8))
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func parseFileActivityQuery(context *gin.Context) (fileActivityQuery, error) {
	limit, err := parseOptionalPositiveInt(context.Query("limit"), fileActivityDefaultLimit)
	if err != nil {
		return fileActivityQuery{}, fmt.Errorf("invalid limit")
	}
	if limit > fileActivityMaxLimit {
		limit = fileActivityMaxLimit
	}

	hours, err := parseOptionalPositiveInt(context.Query("hours"), fileActivityDefaultHours)
	if err != nil {
		return fileActivityQuery{}, fmt.Errorf("invalid hours")
	}
	if hours > fileActivityMaxHours {
		hours = fileActivityMaxHours
	}

	action := strings.ToLower(strings.TrimSpace(context.Query("action")))
	if action != "" {
		switch action {
		case "read", "write", "delete", "permission-change", "share-access", "other":
		default:
			return fileActivityQuery{}, fmt.Errorf("invalid action")
		}
	}

	return fileActivityQuery{
		Limit:  limit,
		Hours:  hours,
		Path:   strings.TrimSpace(context.Query("path")),
		User:   strings.TrimSpace(context.Query("user")),
		Action: action,
	}, nil
}

func fileActivityQueryFromRequest(request fileActivityQueryRequest) (fileActivityQuery, error) {
	limit := request.Limit
	if limit < 1 {
		limit = fileActivityDefaultLimit
	}
	if limit > fileActivityMaxLimit {
		limit = fileActivityMaxLimit
	}

	hours := request.Hours
	if hours < 1 {
		hours = fileActivityDefaultHours
	}
	if hours > fileActivityMaxHours {
		hours = fileActivityMaxHours
	}

	action := strings.ToLower(strings.TrimSpace(request.Action))
	if action != "" {
		switch action {
		case "read", "write", "delete", "permission-change", "share-access", "other":
		default:
			return fileActivityQuery{}, fmt.Errorf("invalid action")
		}
	}

	return fileActivityQuery{
		Limit:  limit,
		Hours:  hours,
		Path:   strings.TrimSpace(request.Path),
		User:   strings.TrimSpace(request.User),
		Action: action,
	}, nil
}

func parseOptionalPositiveInt(raw string, fallback int) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil || value < 1 {
		return 0, errors.New("expected positive integer")
	}
	return value, nil
}

func queryWindowsSecurityFileActivityEvents(parent context.Context, query fileActivityQuery) ([]fileActivityEvent, fileActivitySource, error) {
	source := buildFileActivitySource(query)
	if runtime.GOOS != "windows" {
		return nil, source, errFileActivityUnsupported
	}

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, "wevtutil", "qe", "Security", "/q:"+fileActivityXPath(query.Hours), "/f:xml", "/rd:true", fmt.Sprintf("/c:%d", query.Limit))
	output, err := command.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, source, fmt.Errorf("windows security event log query timed out")
	}
	if err != nil {
		message := strings.TrimSpace(decodeWindowsCommandOutput(output))
		if strings.Contains(strings.ToLower(message), "no events") {
			return []fileActivityEvent{}, source, nil
		}
		if message == "" {
			message = err.Error()
		}
		return nil, source, fmt.Errorf("windows security event log unavailable: %s", sanitizeWindowsEventLogError(message))
	}

	items, err := parseWindowsFileActivityXML(output)
	if err != nil {
		return nil, source, err
	}
	items = filterFileActivityEvents(items, query)
	if len(items) > query.Limit {
		items = items[:query.Limit]
	}
	return items, source, nil
}

func buildFileActivitySource(query fileActivityQuery) fileActivitySource {
	return fileActivitySource{
		Provider:        "Windows Security Event Log",
		Scope:           "local host security log",
		Hours:           query.Hours,
		Limit:           query.Limit,
		ContentScanning: false,
		Requires:        "Advanced Audit Policy Object Access plus SACL auditing on the target folders or shares; the API process must be able to read the Security log.",
		RemoteNote:      "For remote file servers, run OpenAD on the file server or forward Windows Security events into this host before querying.",
		ADResolution:    "not-requested",
	}
}

func fileActivityXPath(hours int) string {
	if hours < 1 {
		hours = fileActivityDefaultHours
	}
	windowMS := hours * 60 * 60 * 1000
	return fmt.Sprintf("*[System[(EventID=4656 or EventID=4660 or EventID=4663 or EventID=4670 or EventID=5140 or EventID=5145) and TimeCreated[timediff(@SystemTime) <= %d]]]", windowMS)
}

func parseWindowsFileActivityXML(raw []byte) ([]fileActivityEvent, error) {
	trimmed := bytes.TrimSpace([]byte(decodeWindowsCommandOutput(raw)))
	if len(trimmed) == 0 {
		return []fileActivityEvent{}, nil
	}

	wrapped := append([]byte("<Events>"), trimmed...)
	wrapped = append(wrapped, []byte("</Events>")...)

	var parsed windowsEventsXML
	if err := xml.Unmarshal(wrapped, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse Windows Security event XML: %w", err)
	}

	items := make([]fileActivityEvent, 0, len(parsed.Events))
	for _, item := range parsed.Events {
		data := windowsEventDataMap(item)
		event := fileActivityEventFromWindows(item, data)
		if event.Path == "" && event.User == "" {
			continue
		}
		items = append(items, event)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Timestamp.After(items[j].Timestamp)
	})
	return items, nil
}

func decodeWindowsCommandOutput(raw []byte) string {
	if len(raw) >= 2 {
		if raw[0] == 0xff && raw[1] == 0xfe {
			return utf16BytesToString(raw[2:], true)
		}
		if raw[0] == 0xfe && raw[1] == 0xff {
			return utf16BytesToString(raw[2:], false)
		}
	}

	if looksLikeUTF16LE(raw) {
		return utf16BytesToString(raw, true)
	}

	return string(raw)
}

func looksLikeUTF16LE(raw []byte) bool {
	if len(raw) < 4 {
		return false
	}
	zeroOddBytes := 0
	checkedOddBytes := 0
	for index := 1; index < len(raw); index += 2 {
		checkedOddBytes++
		if raw[index] == 0x00 {
			zeroOddBytes++
		}
	}
	return checkedOddBytes > 0 && zeroOddBytes*2 > checkedOddBytes
}

func utf16BytesToString(raw []byte, littleEndian bool) string {
	units := make([]uint16, 0, len(raw)/2)
	for index := 0; index+1 < len(raw); index += 2 {
		if littleEndian {
			units = append(units, uint16(raw[index])|uint16(raw[index+1])<<8)
		} else {
			units = append(units, uint16(raw[index])<<8|uint16(raw[index+1]))
		}
	}
	return string(utf16.Decode(units))
}

func sanitizeWindowsEventLogError(message string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" || strings.ContainsRune(trimmed, '\ufffd') {
		return "Security log query failed. Run the API as Administrator or Event Log Readers, then confirm Windows Object Access auditing and folder/share SACLs are configured."
	}
	if len(trimmed) > 500 {
		return trimmed[:500] + "..."
	}
	return trimmed
}

func windowsEventDataMap(item windowsEventXML) map[string]string {
	data := make(map[string]string, len(item.EventData.Data))
	for _, entry := range item.EventData.Data {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		data[name] = strings.TrimSpace(entry.Value)
	}
	return data
}

func fileActivityEventFromWindows(item windowsEventXML, data map[string]string) fileActivityEvent {
	domain := firstNonEmpty(data["SubjectDomainName"], data["AccountDomain"])
	username := firstNonEmpty(data["SubjectUserName"], data["AccountName"])
	userSID := firstNonEmpty(data["SubjectUserSid"], data["TargetSid"], data["MemberSid"], data["Security ID"])
	user := username
	if domain != "" && username != "" && !strings.Contains(username, "\\") {
		user = domain + "\\" + username
	}
	if user == "" {
		user = userSID
	}

	path := firstNonEmpty(data["ObjectName"], data["ShareLocalPath"])
	if path == "" {
		path = sharePathFromEventData(data)
	}

	timestamp, _ := time.Parse(time.RFC3339Nano, item.System.TimeCreated.SystemTime)
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	accessMask := firstNonEmpty(data["AccessMask"], data["Accesses"])
	accessList := firstNonEmpty(data["AccessList"], data["Accesses"])

	return fileActivityEvent{
		EventID:     item.System.EventID,
		Action:      classifyFileActivityAction(item.System.EventID, accessMask, accessList),
		User:        user,
		RawUser:     user,
		UserSID:     userSID,
		Resolution:  "event-log",
		Domain:      domain,
		Path:        path,
		ObjectType:  firstNonEmpty(data["ObjectType"], data["ObjectServer"]),
		AccessMask:  accessMask,
		AccessList:  accessList,
		ProcessName: firstNonEmpty(data["ProcessName"], data["ProcessId"]),
		ClientIP:    firstNonEmpty(data["IpAddress"], data["ClientAddress"]),
		Computer:    strings.TrimSpace(item.System.Computer),
		Timestamp:   timestamp.UTC(),
		Source:      firstNonEmpty(item.System.Provider.Name, "Microsoft-Windows-Security-Auditing"),
	}
}

func (application *application) resolveFileActivityADIdentities(parent context.Context, items []fileActivityEvent, source fileActivitySource, request fileActivityADRequest) ([]fileActivityEvent, fileActivitySource) {
	if len(items) == 0 {
		if request.Enabled {
			source.ADResolution = "requested-no-events"
		}
		return items, source
	}
	if !request.Enabled {
		source.ADResolution = "not-requested"
		return items, source
	}
	if !hasFileActivityADConfig(request) {
		source.ADResolution = "missing-config"
		source.ADWarning = "AD resolution requires server, base DN, username, and password from the current AD workspace session."
		return items, source
	}

	client, err := application.ad.NewGroupClient(request.Server, request.BaseDN, request.Username, request.Password)
	if err != nil {
		source.ADResolution = "failed"
		source.ADWarning = fmt.Sprintf("failed to connect to Active Directory for file activity identity resolution: %v", err)
		return items, source
	}
	defer client.Close()

	cache := make(map[string]*models.ADPrincipal)
	resolved := 0
	unresolved := 0
	for index := range items {
		principal := resolveFileActivityPrincipal(parent, client, cache, items[index])
		if principal == nil {
			unresolved++
			if strings.TrimSpace(items[index].Resolution) == "" {
				items[index].Resolution = "unresolved"
			}
			continue
		}

		resolved++
		applyFileActivityPrincipal(&items[index], *principal)
	}

	source.ADResolution = "enabled"
	source.ADResolved = resolved
	source.ADUnresolved = unresolved
	if unresolved > 0 {
		source.ADWarning = fmt.Sprintf("%d event identities were not resolved by Active Directory and remain in event-log form.", unresolved)
	}
	return items, source
}

func hasFileActivityADConfig(request fileActivityADRequest) bool {
	return strings.TrimSpace(request.Server) != "" &&
		strings.TrimSpace(request.BaseDN) != "" &&
		strings.TrimSpace(request.Username) != "" &&
		strings.TrimSpace(request.Password) != ""
}

func resolveFileActivityPrincipal(ctx context.Context, client adGroupClient, cache map[string]*models.ADPrincipal, event fileActivityEvent) *models.ADPrincipal {
	for _, candidate := range fileActivityIdentityCandidates(event) {
		key := strings.ToLower(strings.TrimSpace(candidate))
		if key == "" {
			continue
		}
		if principal, found := cache[key]; found {
			return principal
		}

		principal, err := client.ResolvePrincipal(ctx, candidate)
		if err != nil || principal == nil || strings.TrimSpace(principal.SAMAccountName+principal.Name+principal.SID+principal.DN) == "" {
			cache[key] = nil
			continue
		}
		copyPrincipal := *principal
		cache[key] = &copyPrincipal
		return &copyPrincipal
	}
	return nil
}

func fileActivityIdentityCandidates(event fileActivityEvent) []string {
	return dedupeFileActivityStrings([]string{
		event.UserSID,
		event.RawUser,
		event.User,
		strings.TrimSpace(event.Domain) + `\` + accountNameFromFileActivityUser(event.User),
		accountNameFromFileActivityUser(event.User),
	})
}

func dedupeFileActivityStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || trimmed == `\` || trimmed == "-" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func accountNameFromFileActivityUser(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || isWindowsSID(trimmed) {
		return ""
	}
	if index := strings.LastIndex(trimmed, `\`); index >= 0 && index < len(trimmed)-1 {
		return strings.TrimSpace(trimmed[index+1:])
	}
	if index := strings.LastIndex(trimmed, "@"); index > 0 {
		return strings.TrimSpace(trimmed[:index])
	}
	return trimmed
}

func applyFileActivityPrincipal(event *fileActivityEvent, principal models.ADPrincipal) {
	account := strings.TrimSpace(principal.SAMAccountName)
	domain := strings.TrimSpace(principal.Domain)
	if domain != "" && account != "" {
		event.User = domain + `\` + account
	} else if account != "" {
		event.User = account
	} else if strings.TrimSpace(principal.Name) != "" {
		event.User = strings.TrimSpace(principal.Name)
	}

	if name := strings.TrimSpace(principal.Name); name != "" && account != "" && !strings.EqualFold(name, account) {
		event.UserDisplay = name
		event.User = event.User + " (" + name + ")"
	} else if strings.TrimSpace(principal.Name) != "" {
		event.UserDisplay = strings.TrimSpace(principal.Name)
	}
	if principal.SID != "" {
		event.UserSID = principal.SID
	}
	if principal.DN != "" {
		event.UserDN = principal.DN
	}
	if principal.Type != "" {
		event.UserType = string(principal.Type)
	}
	if domain != "" {
		event.Domain = domain
	}
	event.Resolution = "active-directory"
}

func isWindowsSID(value string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(value)), "S-1-")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" && trimmed != "-" {
			return trimmed
		}
	}
	return ""
}

func sharePathFromEventData(data map[string]string) string {
	share := firstNonEmpty(data["ShareName"])
	relative := firstNonEmpty(data["RelativeTargetName"])
	if share == "" {
		return relative
	}
	if relative == "" {
		return share
	}
	return strings.TrimRight(share, "\\/") + "\\" + strings.TrimLeft(relative, "\\/")
}

func classifyFileActivityAction(eventID int, accessMask string, accessList string) string {
	switch eventID {
	case 4660:
		return "delete"
	case 4670:
		return "permission-change"
	case 5140:
		return "share-access"
	}

	mask, maskErr := strconv.ParseInt(strings.TrimSpace(accessMask), 0, 64)
	if maskErr == nil {
		if mask&(0x100|0x10000) != 0 {
			return "delete"
		}
		if mask&(0x2|0x4|0x10|0x40|0x40000|0x80000) != 0 {
			return "write"
		}
		if mask&(0x1|0x20|0x80|0x20000) != 0 {
			return "read"
		}
	}

	lower := strings.ToLower(accessList)
	switch {
	case strings.Contains(lower, "delete"):
		return "delete"
	case strings.Contains(lower, "write") || strings.Contains(lower, "append") || strings.Contains(lower, "add file") || strings.Contains(lower, "change permissions") || strings.Contains(lower, "take ownership"):
		return "write"
	case strings.Contains(lower, "read") || strings.Contains(lower, "list") || strings.Contains(lower, "execute"):
		return "read"
	case eventID == 5145:
		return "share-access"
	default:
		return "other"
	}
}

func filterFileActivityEvents(items []fileActivityEvent, query fileActivityQuery) []fileActivityEvent {
	pathFilter := strings.ToLower(strings.TrimSpace(query.Path))
	userFilter := strings.ToLower(strings.TrimSpace(query.User))
	result := make([]fileActivityEvent, 0, len(items))
	for _, item := range items {
		if pathFilter != "" && !strings.Contains(strings.ToLower(item.Path), pathFilter) {
			continue
		}
		if userFilter != "" && !strings.Contains(strings.ToLower(item.User), userFilter) {
			continue
		}
		if query.Action != "" && item.Action != query.Action {
			continue
		}
		result = append(result, item)
	}
	return result
}

func summarizeFileActivityEvents(items []fileActivityEvent) fileActivitySummary {
	summary := fileActivitySummary{Total: len(items)}
	for _, item := range items {
		switch item.Action {
		case "read":
			summary.Read++
		case "write":
			summary.Write++
		case "delete":
			summary.Delete++
		case "permission-change":
			summary.PermissionChanges++
		case "share-access":
			summary.ShareAccess++
		default:
			summary.Other++
		}
	}
	return summary
}
