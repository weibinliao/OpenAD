package main

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var auditEntries = newAuditBuffer(1500)

type AuditSummaryExportRequest struct {
	RequestID string `json:"request_id"`
	Path      string `json:"path"`
	Method    string `json:"method"`
	Status    int    `json:"status"`
	Title     string `json:"title"`
}

func (application *application) handleListAuditRequests(context *gin.Context) {
	page, err := parsePositiveIntQuery(context, "page")
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pageSize, err := parsePositiveIntQuery(context, "page_size")
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	filters, err := parseAuditQueryFilters(context)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filtered := filterAuditEntries(auditEntries.List(), filters)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	start := (page - 1) * pageSize
	total := len(filtered)
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items := filtered[start:end]
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	context.JSON(http.StatusOK, gin.H{
		"items": items,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

func (application *application) handleGetAuditRequest(context *gin.Context) {
	requestID := strings.TrimSpace(context.Param("request_id"))
	if requestID == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "request_id is required"})
		return
	}

	page, err := parsePositiveIntQuery(context, "page")
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pageSize, err := parsePositiveIntQuery(context, "page_size")
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 500 {
		pageSize = 500
	}

	filters, err := parseAuditQueryFilters(context)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	filters.RequestID = requestID

	matched := filterAuditEntries(auditEntries.List(), filters)
	if len(matched) == 0 {
		context.JSON(http.StatusNotFound, gin.H{"error": "audit request not found"})
		return
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Timestamp.After(matched[j].Timestamp)
	})

	start := (page - 1) * pageSize
	total := len(matched)
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items := matched[start:end]
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	context.JSON(http.StatusOK, gin.H{
		"request_id":  requestID,
		"count":       len(matched),
		"total_count": total,
		"entries":     items,
		"latest":      matched[0],
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

func (application *application) handleExportAuditSummary(context *gin.Context) {
	var request AuditSummaryExportRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if request.Status != 0 && (request.Status < 100 || request.Status > 599) {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}

	filters := auditQueryFilters{
		RequestID: strings.TrimSpace(request.RequestID),
		Path:      strings.ToLower(strings.TrimSpace(request.Path)),
		Method:    strings.ToUpper(strings.TrimSpace(request.Method)),
		Status:    request.Status,
	}
	filtered := filterAuditEntries(auditEntries.List(), filters)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = "OpenAD - Audit Summary"
	}
	markdown := renderAuditSummaryMarkdown(title, filtered, filters)
	context.Header("Content-Type", "text/markdown; charset=utf-8")
	context.Header("Content-Disposition", `attachment; filename="audit-summary.md"`)
	context.String(http.StatusOK, markdown)
}

type auditQueryFilters struct {
	RequestID string
	Path      string
	Method    string
	Status    int
}

func parseAuditQueryFilters(context *gin.Context) (auditQueryFilters, error) {
	requestID := strings.TrimSpace(context.Query("request_id"))
	pathFilter := strings.ToLower(strings.TrimSpace(context.Query("path")))
	methodFilter := strings.ToUpper(strings.TrimSpace(context.Query("method")))
	statusFilterRaw := strings.TrimSpace(context.Query("status"))
	statusFilter := 0
	if methodFilter != "" {
		switch methodFilter {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodHead, http.MethodOptions:
		default:
			return auditQueryFilters{}, errors.New("invalid method")
		}
	}
	if statusFilterRaw != "" {
		parsed, parseErr := strconv.Atoi(statusFilterRaw)
		if parseErr != nil || parsed < 100 || parsed > 599 {
			return auditQueryFilters{}, errors.New("invalid status")
		}
		statusFilter = parsed
	}

	return auditQueryFilters{
		RequestID: requestID,
		Path:      pathFilter,
		Method:    methodFilter,
		Status:    statusFilter,
	}, nil
}

func filterAuditEntries(entries []auditEntry, filters auditQueryFilters) []auditEntry {
	filtered := make([]auditEntry, 0, len(entries))
	for _, entry := range entries {
		if filters.RequestID != "" && !strings.EqualFold(entry.RequestID, filters.RequestID) {
			continue
		}
		if filters.Path != "" && !strings.Contains(strings.ToLower(entry.Path), filters.Path) {
			continue
		}
		if filters.Method != "" && !strings.EqualFold(entry.Method, filters.Method) {
			continue
		}
		if filters.Status != 0 && entry.Status != filters.Status {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func renderAuditSummaryMarkdown(title string, entries []auditEntry, filters auditQueryFilters) string {
	var builder strings.Builder
	builder.WriteString("# ")
	builder.WriteString(title)
	builder.WriteString("\n\n")
	builder.WriteString(fmt.Sprintf("- Generated At: %s\n", time.Now().UTC().Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- Total Requests: %d\n", len(entries)))
	if filters.RequestID != "" {
		builder.WriteString(fmt.Sprintf("- Request ID: `%s`\n", filters.RequestID))
	}
	if filters.Path != "" {
		builder.WriteString(fmt.Sprintf("- Path Filter: `%s`\n", filters.Path))
	}
	if filters.Method != "" {
		builder.WriteString(fmt.Sprintf("- Method Filter: `%s`\n", filters.Method))
	}
	if filters.Status != 0 {
		builder.WriteString(fmt.Sprintf("- Status Filter: `%d`\n", filters.Status))
	}
	builder.WriteString("\n## Top Endpoints\n")

	if len(entries) == 0 {
		builder.WriteString("\nNo matched audit records.\n")
		return builder.String()
	}

	type endpointCount struct {
		Path  string
		Count int
	}

	statusCounts := map[int]int{}
	pathCounts := map[string]int{}
	maxDuration := entries[0]
	for _, entry := range entries {
		statusCounts[entry.Status]++
		pathCounts[entry.Path]++
		if entry.DurationMS > maxDuration.DurationMS {
			maxDuration = entry
		}
	}

	endpoints := make([]endpointCount, 0, len(pathCounts))
	for path, count := range pathCounts {
		endpoints = append(endpoints, endpointCount{Path: path, Count: count})
	}
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Count == endpoints[j].Count {
			return endpoints[i].Path < endpoints[j].Path
		}
		return endpoints[i].Count > endpoints[j].Count
	})
	if len(endpoints) > 10 {
		endpoints = endpoints[:10]
	}
	for _, endpoint := range endpoints {
		builder.WriteString(fmt.Sprintf("- `%s`: %d\n", endpoint.Path, endpoint.Count))
	}

	builder.WriteString("\n## Status Distribution\n")
	statusCodes := make([]int, 0, len(statusCounts))
	for status := range statusCounts {
		statusCodes = append(statusCodes, status)
	}
	sort.Ints(statusCodes)
	for _, status := range statusCodes {
		builder.WriteString(fmt.Sprintf("- `%d`: %d\n", status, statusCounts[status]))
	}

	builder.WriteString("\n## Slowest Request\n")
	builder.WriteString(fmt.Sprintf("- Request ID: `%s`\n", maxDuration.RequestID))
	builder.WriteString(fmt.Sprintf("- Path: `%s`\n", maxDuration.Path))
	builder.WriteString(fmt.Sprintf("- Duration: `%d ms`\n", maxDuration.DurationMS))
	builder.WriteString(fmt.Sprintf("- Timestamp: `%s`\n", maxDuration.Timestamp.Format(time.RFC3339)))

	return builder.String()
}

type auditEntry struct {
	RequestID  string    `json:"request_id"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Status     int       `json:"status"`
	DurationMS int64     `json:"duration_ms"`
	ClientIP   string    `json:"client_ip"`
	UserAgent  string    `json:"user_agent"`
	Timestamp  time.Time `json:"timestamp"`
}

type auditBuffer struct {
	limit   int
	entries []auditEntry
	mutex   sync.RWMutex
}

func newAuditBuffer(limit int) *auditBuffer {
	if limit < 1 {
		limit = 200
	}

	return &auditBuffer{
		limit:   limit,
		entries: make([]auditEntry, 0, limit),
	}
}

func (buffer *auditBuffer) Add(entry auditEntry) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()

	if len(buffer.entries) >= buffer.limit {
		copy(buffer.entries, buffer.entries[1:])
		buffer.entries[len(buffer.entries)-1] = entry
		return
	}

	buffer.entries = append(buffer.entries, entry)
}

func (buffer *auditBuffer) List() []auditEntry {
	buffer.mutex.RLock()
	defer buffer.mutex.RUnlock()

	return append([]auditEntry(nil), buffer.entries...)
}
