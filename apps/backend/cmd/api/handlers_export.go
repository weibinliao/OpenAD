package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/weibinliao/OpenAD/internal/export"
	"github.com/weibinliao/OpenAD/internal/models"
)

type ExportRequest struct {
	Permissions  []models.Permission `json:"permissions" binding:"required"`
	UserRows     []export.UserRow    `json:"user_rows"`
	Mode         string              `json:"export_mode"`
	Format       string              `json:"format" binding:"required"`
	Filename     string              `json:"filename" binding:"required"`
	Title        string              `json:"title"`
	Template     string              `json:"template"`
	Sections     []string            `json:"sections"`
	Organization string              `json:"organization"`
	PreparedBy   string              `json:"prepared_by"`
	ReportPeriod string              `json:"report_period"`
	FocusAreas   []string            `json:"focus_areas"`
	ADFields     []string            `json:"ad_fields"`
	FileColumns  []string            `json:"file_columns"`
}

type ExportDownloadRequest struct {
	Permissions  []models.Permission `json:"permissions" binding:"required"`
	UserRows     []export.UserRow    `json:"user_rows"`
	Mode         string              `json:"export_mode"`
	Format       string              `json:"format" binding:"required"`
	Filename     string              `json:"filename"`
	Title        string              `json:"title"`
	Template     string              `json:"template"`
	Sections     []string            `json:"sections"`
	Organization string              `json:"organization"`
	PreparedBy   string              `json:"prepared_by"`
	ReportPeriod string              `json:"report_period"`
	FocusAreas   []string            `json:"focus_areas"`
	ADFields     []string            `json:"ad_fields"`
	FileColumns  []string            `json:"file_columns"`
}

type ExportSummaryRequest struct {
	Permissions  []models.Permission `json:"permissions" binding:"required"`
	UserRows     []export.UserRow    `json:"user_rows"`
	Mode         string              `json:"export_mode"`
	Title        string              `json:"title"`
	Template     string              `json:"template"`
	Sections     []string            `json:"sections"`
	Organization string              `json:"organization"`
	PreparedBy   string              `json:"prepared_by"`
	ReportPeriod string              `json:"report_period"`
	FocusAreas   []string            `json:"focus_areas"`
	ADFields     []string            `json:"ad_fields"`
	FileColumns  []string            `json:"file_columns"`
}

const exportRequestBodyLimitBytes int64 = 64 << 20

func (application *application) handleExport(context *gin.Context) {
	var request ExportRequest
	if !bindExportJSON(context, &request, exportRequestBodyLimitBytes) {
		return
	}

	format := strings.ToLower(strings.TrimSpace(request.Format))
	extension, _ := exportFormatMeta(format)
	if extension == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "unsupported format"})
		return
	}

	exportPath, err := resolveServerExportPath(request.Filename, extension)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	options := export.Options{
		Title:        request.Title,
		Mode:         request.Mode,
		Template:     request.Template,
		Sections:     append([]string(nil), request.Sections...),
		Organization: request.Organization,
		PreparedBy:   request.PreparedBy,
		ReportPeriod: request.ReportPeriod,
		FocusAreas:   append([]string(nil), request.FocusAreas...),
		ADFields:     append([]string(nil), request.ADFields...),
		FileColumns:  append([]string(nil), request.FileColumns...),
		UserRows:     append([]export.UserRow(nil), request.UserRows...),
	}

	switch format {
	case "csv":
		err = application.exporter.ExportToCSV(request.Permissions, exportPath, options)
	case "excel":
		err = application.exporter.ExportToExcel(request.Permissions, exportPath, options)
	case "html":
		err = application.exporter.ExportToHTML(request.Permissions, exportPath, options)
	}

	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	context.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Export completed: %s", exportPath), "path": exportPath})
}

func (application *application) handleExportDownload(context *gin.Context) {
	var request ExportDownloadRequest
	if !bindExportJSON(context, &request, exportRequestBodyLimitBytes) {
		return
	}

	format := strings.ToLower(strings.TrimSpace(request.Format))
	extension, contentType := exportFormatMeta(format)
	if extension == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "unsupported format"})
		return
	}

	baseName := sanitizeExportFilename(request.Filename, extension)

	tempFile, err := os.CreateTemp("", "fsa-export-*."+extension)
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()
	defer func() { _ = os.Remove(tempPath) }()
	options := export.Options{
		Title:        request.Title,
		Mode:         request.Mode,
		Template:     request.Template,
		Sections:     append([]string(nil), request.Sections...),
		Organization: request.Organization,
		PreparedBy:   request.PreparedBy,
		ReportPeriod: request.ReportPeriod,
		FocusAreas:   append([]string(nil), request.FocusAreas...),
		ADFields:     append([]string(nil), request.ADFields...),
		FileColumns:  append([]string(nil), request.FileColumns...),
		UserRows:     append([]export.UserRow(nil), request.UserRows...),
	}

	switch format {
	case "csv":
		err = application.exporter.ExportToCSV(request.Permissions, tempPath, options)
	case "excel":
		err = application.exporter.ExportToExcel(request.Permissions, tempPath, options)
	case "html":
		err = application.exporter.ExportToHTML(request.Permissions, tempPath, options)
	default:
		context.JSON(http.StatusBadRequest, gin.H{"error": "unsupported format"})
		return
	}
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	context.Header("Content-Type", contentType)
	context.Header("Content-Disposition", exportContentDisposition(baseName))
	context.File(tempPath)
}

func (application *application) handleExportSummary(context *gin.Context) {
	var request ExportSummaryRequest
	if !bindExportJSON(context, &request, exportRequestBodyLimitBytes) {
		return
	}

	templateID := normalizeSummaryTemplateID(request.Template)
	template, ok := findSummaryTemplateByID(templateID)
	if !ok {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid summary template"})
		return
	}

	summary := buildPermissionSummary(request.Permissions)
	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = template.DefaultTitle
	}
	metadata := buildSummaryReportMetadata(request)
	sections := resolveSummarySections(template, request.Sections)
	markdown := renderSummaryMarkdownByTemplate(template.ID, title, summary, metadata, sections)

	context.JSON(http.StatusOK, gin.H{
		"title":         title,
		"summary":       summary,
		"metadata":      metadata,
		"sections":      sections,
		"markdown":      markdown,
		"template":      template.ID,
		"template_name": template.Name,
	})
}

func bindExportJSON(context *gin.Context, destination any, limitBytes int64) bool {
	if context.Request.ContentLength > limitBytes {
		writeExportRequestTooLarge(context)
		return false
	}

	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, limitBytes)
	if err := context.ShouldBindJSON(destination); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeExportRequestTooLarge(context)
			return false
		}

		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}

	return true
}

func writeExportRequestTooLarge(context *gin.Context) {
	context.JSON(http.StatusRequestEntityTooLarge, gin.H{
		"error": "export request is too large; narrow the scan scope or export permissions in smaller batches",
	})
}

func (application *application) handleListExportSummaryTemplates(context *gin.Context) {
	items := summaryTemplates()
	context.JSON(http.StatusOK, gin.H{
		"items": items,
		"count": len(items),
	})
}

func exportFormatMeta(format string) (extension string, contentType string) {
	switch format {
	case "csv":
		return "csv", "text/csv; charset=utf-8"
	case "excel":
		return "xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "html":
		return "html", "text/html; charset=utf-8"
	default:
		return "", ""
	}
}

func resolveServerExportPath(filename string, extension string) (string, error) {
	baseName := sanitizeExportFilename(filename, extension)

	exportDir := strings.TrimSpace(os.Getenv("PERMISSION_PROTECTOR_EXPORT_DIR"))
	if exportDir == "" {
		exportDir = strings.TrimSpace(os.Getenv("PERMISSION_PROTECTOR_DATA_DIR"))
		if exportDir == "" {
			if configDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(configDir) != "" {
				exportDir = filepath.Join(configDir, "PermissionProtector")
			}
		}
		if exportDir == "" {
			exportDir = filepath.Join(os.TempDir(), "PermissionProtector")
		}
		exportDir = filepath.Join(exportDir, "exports")
	}

	if err := os.MkdirAll(exportDir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(exportDir, baseName), nil
}

func sanitizeExportFilename(filename string, extension string) string {
	baseName := strings.TrimSpace(filepath.Base(filename))
	if baseName == "" || baseName == "." || baseName == string(filepath.Separator) {
		baseName = "permissions-export"
	}

	baseName = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '-'
		}
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*', ';':
			return '-'
		default:
			return r
		}
	}, baseName)
	baseName = strings.Trim(baseName, " .")
	if baseName == "" {
		baseName = "permissions-export"
	}
	if !strings.HasSuffix(strings.ToLower(baseName), "."+extension) {
		baseName = fmt.Sprintf("%s.%s", strings.TrimSuffix(baseName, "."), extension)
	}
	return baseName
}

func exportContentDisposition(filename string) string {
	return fmt.Sprintf(
		"attachment; filename=\"%s\"; filename*=UTF-8''%s",
		asciiExportFilename(filename),
		encodeRFC5987Value(filename),
	)
}

func encodeRFC5987Value(value string) string {
	const hex = "0123456789ABCDEF"
	var encoded strings.Builder
	encoded.Grow(len(value))
	for _, char := range []byte(value) {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || strings.ContainsRune("!#$&+-.^_`|~", rune(char)) {
			encoded.WriteByte(char)
			continue
		}
		encoded.WriteByte('%')
		encoded.WriteByte(hex[char>>4])
		encoded.WriteByte(hex[char&0x0f])
	}
	return encoded.String()
}

func asciiExportFilename(filename string) string {
	extension := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, extension)
	stem = strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e {
			return '-'
		}
		return r
	}, stem)
	stem = strings.Trim(stem, " .-")
	if stem == "" {
		stem = "permissions-export"
	}
	return stem + extension
}

type permissionSummary struct {
	TotalPermissions int           `json:"total_permissions"`
	ExplicitCount    int           `json:"explicit_count"`
	InheritedCount   int           `json:"inherited_count"`
	DenyCount        int           `json:"deny_count"`
	HighRiskCount    int           `json:"high_risk_count"`
	UniquePaths      int           `json:"unique_paths"`
	UniqueTrustees   int           `json:"unique_trustees"`
	TopTrustees      []summaryItem `json:"top_trustees"`
	TopPaths         []summaryItem `json:"top_paths"`
}

type summaryItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type summaryReportMetadata struct {
	Organization string   `json:"organization"`
	PreparedBy   string   `json:"prepared_by"`
	ReportPeriod string   `json:"report_period"`
	FocusAreas   []string `json:"focus_areas"`
	GeneratedAt  string   `json:"generated_at"`
}

type reportSummaryTemplate struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	DefaultTitle      string   `json:"default_title"`
	AvailableSections []string `json:"available_sections"`
	DefaultSections   []string `json:"default_sections"`
}

func summaryTemplates() []reportSummaryTemplate {
	return []reportSummaryTemplate{
		{
			ID:                "management",
			Name:              "Management",
			Description:       "Executive overview with key risk indicators and volume trends.",
			DefaultTitle:      "OpenAD - Management Summary",
			AvailableSections: []string{"metadata", "kpis", "top_trustees", "top_paths"},
			DefaultSections:   []string{"metadata", "kpis", "top_trustees", "top_paths"},
		},
		{
			ID:                "compliance",
			Name:              "Compliance",
			Description:       "Control-oriented report for audit evidence and policy checks.",
			DefaultTitle:      "OpenAD - Compliance Summary",
			AvailableSections: []string{"metadata", "control_snapshot", "audit_coverage", "risk_principals", "risk_paths", "recommendations"},
			DefaultSections:   []string{"metadata", "control_snapshot", "audit_coverage", "risk_principals", "risk_paths", "recommendations"},
		},
		{
			ID:                "operations",
			Name:              "Operations",
			Description:       "Operations-focused report for remediation and ownership planning.",
			DefaultTitle:      "OpenAD - Operations Summary",
			AvailableSections: []string{"metadata", "operations_kpis", "top_owners", "top_paths", "action_plan"},
			DefaultSections:   []string{"metadata", "operations_kpis", "top_owners", "top_paths", "action_plan"},
		},
	}
}

func normalizeSummaryTemplateID(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return "management"
	}

	return normalized
}

func findSummaryTemplateByID(templateID string) (reportSummaryTemplate, bool) {
	for _, item := range summaryTemplates() {
		if strings.EqualFold(item.ID, templateID) {
			return item, true
		}
	}

	return reportSummaryTemplate{}, false
}

func resolveSummarySections(template reportSummaryTemplate, requested []string) []string {
	if len(requested) == 0 {
		return append([]string(nil), template.DefaultSections...)
	}

	allowed := make(map[string]struct{}, len(template.AvailableSections))
	for _, item := range template.AvailableSections {
		allowed[strings.ToLower(strings.TrimSpace(item))] = struct{}{}
	}

	result := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, item := range requested {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized == "" {
			continue
		}
		if _, ok := allowed[normalized]; !ok {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	if len(result) == 0 {
		return append([]string(nil), template.DefaultSections...)
	}

	return result
}

func summarySectionSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized != "" {
			set[normalized] = struct{}{}
		}
	}

	return set
}

func hasSummarySection(sectionSet map[string]struct{}, section string) bool {
	_, ok := sectionSet[strings.ToLower(strings.TrimSpace(section))]
	return ok
}

func buildPermissionSummary(permissions []models.Permission) permissionSummary {
	summary := permissionSummary{TotalPermissions: len(permissions)}
	pathCounts := make(map[string]int)
	trusteeCounts := make(map[string]int)
	uniquePaths := make(map[string]struct{})
	uniqueTrustees := make(map[string]struct{})

	for _, permission := range permissions {
		if permission.Inherited {
			summary.InheritedCount++
		} else {
			summary.ExplicitCount++
		}
		if strings.EqualFold(strings.TrimSpace(permission.Type), "deny") {
			summary.DenyCount++
		}
		if isHighRiskPermission(permission.Rights) {
			summary.HighRiskCount++
		}

		path := strings.TrimSpace(permission.Path)
		if path != "" {
			pathCounts[path]++
			uniquePaths[strings.ToLower(path)] = struct{}{}
		}

		trustee := strings.TrimSpace(permission.Trustee)
		if trustee != "" {
			trusteeCounts[trustee]++
			uniqueTrustees[strings.ToLower(trustee)] = struct{}{}
		}
	}

	summary.UniquePaths = len(uniquePaths)
	summary.UniqueTrustees = len(uniqueTrustees)
	summary.TopTrustees = buildTopSummaryItems(trusteeCounts, 10)
	summary.TopPaths = buildTopSummaryItems(pathCounts, 10)

	return summary
}

func buildTopSummaryItems(counts map[string]int, limit int) []summaryItem {
	items := make([]summaryItem, 0, len(counts))
	for name, count := range counts {
		items = append(items, summaryItem{Name: name, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func buildSummaryReportMetadata(request ExportSummaryRequest) summaryReportMetadata {
	focusAreas := make([]string, 0, len(request.FocusAreas))
	for _, item := range request.FocusAreas {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			focusAreas = append(focusAreas, trimmed)
		}
	}
	if len(focusAreas) == 0 {
		focusAreas = []string{"High-risk rights", "Deny precedence", "Explicit assignments"}
	}

	organization := strings.TrimSpace(request.Organization)
	if organization == "" {
		organization = "Internal Security Team"
	}
	preparedBy := strings.TrimSpace(request.PreparedBy)
	if preparedBy == "" {
		preparedBy = "OpenAD"
	}
	reportPeriod := strings.TrimSpace(request.ReportPeriod)
	if reportPeriod == "" {
		reportPeriod = time.Now().UTC().Format("2006-01")
	}

	return summaryReportMetadata{
		Organization: organization,
		PreparedBy:   preparedBy,
		ReportPeriod: reportPeriod,
		FocusAreas:   focusAreas,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
	}
}

func renderSummaryMetadataLines(metadata summaryReportMetadata) []string {
	lines := []string{
		fmt.Sprintf("- Organization: %s", metadata.Organization),
		fmt.Sprintf("- Prepared By: %s", metadata.PreparedBy),
		fmt.Sprintf("- Report Period: %s", metadata.ReportPeriod),
		fmt.Sprintf("- Generated At: %s", metadata.GeneratedAt),
	}
	if len(metadata.FocusAreas) > 0 {
		lines = append(lines, "- Focus Areas:")
		for _, item := range metadata.FocusAreas {
			lines = append(lines, fmt.Sprintf("  - %s", item))
		}
	}

	return lines
}

func renderSummaryMarkdownByTemplate(templateID, title string, summary permissionSummary, metadata summaryReportMetadata, sections []string) string {
	set := summarySectionSet(sections)
	switch normalizeSummaryTemplateID(templateID) {
	case "compliance":
		return renderComplianceSummaryMarkdown(title, summary, metadata, set)
	case "operations":
		return renderOperationsSummaryMarkdown(title, summary, metadata, set)
	default:
		return renderManagementSummaryMarkdown(title, summary, metadata, set)
	}
}

func renderManagementSummaryMarkdown(title string, summary permissionSummary, metadata summaryReportMetadata, sections map[string]struct{}) string {
	lines := []string{
		fmt.Sprintf("# %s", title),
		"",
	}
	if hasSummarySection(sections, "metadata") {
		lines = append(lines, renderSummaryMetadataLines(metadata)...)
		lines = append(lines, "")
	}
	if hasSummarySection(sections, "kpis") {
		lines = append(lines,
			fmt.Sprintf("- Total permissions: %d", summary.TotalPermissions),
			fmt.Sprintf("- Explicit: %d", summary.ExplicitCount),
			fmt.Sprintf("- Inherited: %d", summary.InheritedCount),
			fmt.Sprintf("- Deny entries: %d", summary.DenyCount),
			fmt.Sprintf("- High risk entries: %d", summary.HighRiskCount),
			fmt.Sprintf("- Unique paths: %d", summary.UniquePaths),
			fmt.Sprintf("- Unique trustees: %d", summary.UniqueTrustees),
		)
	}
	if hasSummarySection(sections, "top_trustees") {
		lines = append(lines, "", "## Top Trustees")
		if len(summary.TopTrustees) == 0 {
			lines = append(lines, "- (none)")
		} else {
			for _, item := range summary.TopTrustees {
				lines = append(lines, fmt.Sprintf("- %s: %d", item.Name, item.Count))
			}
		}
	}
	if hasSummarySection(sections, "top_paths") {
		lines = append(lines, "", "## Top Paths")
		if len(summary.TopPaths) == 0 {
			lines = append(lines, "- (none)")
		} else {
			for _, item := range summary.TopPaths {
				lines = append(lines, fmt.Sprintf("- %s: %d", item.Name, item.Count))
			}
		}
	}

	return strings.Join(lines, "\n")
}

func renderComplianceSummaryMarkdown(title string, summary permissionSummary, metadata summaryReportMetadata, sections map[string]struct{}) string {
	lines := []string{
		fmt.Sprintf("# %s", title),
		"",
	}
	if hasSummarySection(sections, "metadata") {
		lines = append(lines, renderSummaryMetadataLines(metadata)...)
		lines = append(lines, "")
	}
	if hasSummarySection(sections, "control_snapshot") {
		lines = append(lines,
			"## Control Snapshot",
			fmt.Sprintf("- High risk entries: %d", summary.HighRiskCount),
			fmt.Sprintf("- Deny entries: %d", summary.DenyCount),
			fmt.Sprintf("- Explicit entries: %d", summary.ExplicitCount),
			fmt.Sprintf("- Inherited entries: %d", summary.InheritedCount),
		)
	}
	if hasSummarySection(sections, "audit_coverage") {
		lines = append(lines,
			"",
			"## Audit Coverage",
			fmt.Sprintf("- Unique paths reviewed: %d", summary.UniquePaths),
			fmt.Sprintf("- Unique trustees reviewed: %d", summary.UniqueTrustees),
		)
	}
	if hasSummarySection(sections, "risk_principals") {
		lines = append(lines, "", "## Primary Risk Principals")
		if len(summary.TopTrustees) == 0 {
			lines = append(lines, "- (none)")
		} else {
			for _, item := range summary.TopTrustees {
				lines = append(lines, fmt.Sprintf("- %s: %d entries", item.Name, item.Count))
			}
		}
	}
	if hasSummarySection(sections, "risk_paths") {
		lines = append(lines, "", "## Primary Risk Paths")
		if len(summary.TopPaths) == 0 {
			lines = append(lines, "- (none)")
		} else {
			for _, item := range summary.TopPaths {
				lines = append(lines, fmt.Sprintf("- %s: %d entries", item.Name, item.Count))
			}
		}
	}
	if hasSummarySection(sections, "recommendations") {
		lines = append(lines, "", "## Recommended Compliance Checks")
		if summary.HighRiskCount > 0 {
			lines = append(lines, "- Validate approval trail for high-risk rights (Full Control/Modify/write owner).")
		}
		if summary.DenyCount > 0 {
			lines = append(lines, "- Confirm deny precedence aligns with least-privilege policy.")
		}
		lines = append(lines, "- Archive this report with scan session IDs for audit evidence.")
	}

	return strings.Join(lines, "\n")
}

func renderOperationsSummaryMarkdown(title string, summary permissionSummary, metadata summaryReportMetadata, sections map[string]struct{}) string {
	lines := []string{
		fmt.Sprintf("# %s", title),
		"",
	}
	if hasSummarySection(sections, "metadata") {
		lines = append(lines, renderSummaryMetadataLines(metadata)...)
		lines = append(lines, "")
	}
	if hasSummarySection(sections, "operations_kpis") {
		lines = append(lines,
			"## Operations KPIs",
			fmt.Sprintf("- Total permissions: %d", summary.TotalPermissions),
			fmt.Sprintf("- High risk backlog: %d", summary.HighRiskCount),
			fmt.Sprintf("- Deny backlog: %d", summary.DenyCount),
			fmt.Sprintf("- Unique trustees: %d", summary.UniqueTrustees),
		)
	}
	if hasSummarySection(sections, "top_owners") {
		lines = append(lines, "", "## Top Owners To Review")
		if len(summary.TopTrustees) == 0 {
			lines = append(lines, "- (none)")
		} else {
			for _, item := range summary.TopTrustees {
				lines = append(lines, fmt.Sprintf("- %s: %d items", item.Name, item.Count))
			}
		}
	}
	if hasSummarySection(sections, "top_paths") {
		lines = append(lines, "", "## Top Paths To Remediate")
		if len(summary.TopPaths) == 0 {
			lines = append(lines, "- (none)")
		} else {
			for _, item := range summary.TopPaths {
				lines = append(lines, fmt.Sprintf("- %s: %d items", item.Name, item.Count))
			}
		}
	}
	if hasSummarySection(sections, "action_plan") {
		lines = append(lines, "", "## Suggested Next Actions")
		lines = append(lines, "- Prioritize paths with repeated high-risk rights and explicit assignments.")
		lines = append(lines, "- Re-run scan after remediation and compare sessions for drift closure.")
	}

	return strings.Join(lines, "\n")
}

func isHighRiskPermission(rights string) bool {
	value := strings.ToLower(strings.TrimSpace(rights))
	if value == "" {
		return false
	}

	tokens := []string{"full control", "modify", "write_dac", "write owner", "take ownership"}
	for _, token := range tokens {
		if strings.Contains(value, token) {
			return true
		}
	}

	return false
}
