package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/weibinliao/OpenAD/internal/ad"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/gin-gonic/gin"
)

var adJobSequence uint64
var adTreeCache = newADTreeResponseCache(15 * time.Second)
var adTreeLimiter = newWindowRateLimiter(40, time.Minute)
var adQueryJobs = newADQueryJobStore(200)

type ADConnectionRequest struct {
	ADCredentials
}

type ADUserQueryRequest struct {
	ADCredentials
	Query string `json:"query" binding:"required"`
	Limit int    `json:"limit"`
}

type ADGroupQueryRequest struct {
	ADCredentials
	Query string `json:"query" binding:"required"`
	Limit int    `json:"limit"`
}

type ADGroupMembersRequest struct {
	ADCredentials
	GroupDN string `json:"group_dn" binding:"required"`

	IncludeNested        bool     `json:"include_nested"`
	MaxDepth             int      `json:"max_depth"`
	ExcludeGroupPatterns []string `json:"exclude_group_patterns"`
	ExcludeUserPatterns  []string `json:"exclude_user_patterns"`
}

type ADTreeRequest struct {
	ADCredentials
	ParentDN  string   `json:"parent_dn"`
	Limit     int      `json:"limit"`
	Page      int      `json:"page"`
	PageSize  int      `json:"page_size"`
	NodeTypes []string `json:"node_types"`
}

type ADTreeExplainRequest struct {
	DN       string `json:"dn" binding:"required"`
	NodeType string `json:"node_type"`
}

type ADPrincipalExpandRequest struct {
	ADCredentials
	Identifiers          []string `json:"identifiers" binding:"required"`
	IncludeNested        bool     `json:"include_nested"`
	MaxDepth             int      `json:"max_depth"`
	ExcludeGroupPatterns []string `json:"exclude_group_patterns"`
	ExcludeUserPatterns  []string `json:"exclude_user_patterns"`
}

type ADUserAsyncQueryRequest struct {
	ADCredentials
	Query string `json:"query" binding:"required"`
	Limit int    `json:"limit"`
}

func (application *application) handleADUserQuery(context *gin.Context) {
	var request ADUserQueryRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := request.resolveInto(); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client, err := application.ad.NewUserSearchClient(request.Server, request.BaseDN, request.Username, request.Password)
	if err != nil {
		context.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to connect to Active Directory: %v", err)})
		return
	}
	defer client.Close()

	users, err := client.SearchUsers(request.Query, request.Limit)
	if err != nil {
		context.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to query Active Directory users: %v", err)})
		return
	}

	context.JSON(http.StatusOK, gin.H{
		"query": strings.TrimSpace(request.Query),
		"count": len(users),
		"users": users,
	})
}

func (application *application) handleADUserQueryAsync(context *gin.Context) {
	var request ADUserAsyncQueryRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := request.resolveInto(); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jobID := fmt.Sprintf("adjob-%d-%d", time.Now().UTC().UnixNano(), atomic.AddUint64(&adJobSequence, 1))
	now := time.Now().UTC()
	adQueryJobs.Set(adQueryJob{
		ID:        jobID,
		Status:    "queued",
		Progress:  0,
		Query:     strings.TrimSpace(request.Query),
		CreatedAt: now,
		UpdatedAt: now,
		Request:   request,
	})

	go application.runADUserQueryJob(jobID, request)

	context.JSON(http.StatusAccepted, gin.H{
		"job_id":  jobID,
		"status":  "queued",
		"message": "AD query job created",
	})
}

func (application *application) runADUserQueryJob(jobID string, request ADUserAsyncQueryRequest) {
	adQueryJobs.Update(jobID, func(job *adQueryJob) {
		if job.CancelRequested {
			job.Status = "cancelled"
			job.Progress = 100
			job.Error = "job cancelled"
			job.UpdatedAt = time.Now().UTC()
			return
		}
		job.Status = "running"
		job.Progress = 10
		job.UpdatedAt = time.Now().UTC()
	})

	if job, ok := adQueryJobs.Get(jobID); ok && job.Status == "cancelled" {
		return
	}

	client, err := application.ad.NewUserSearchClient(request.Server, request.BaseDN, request.Username, request.Password)
	if err != nil {
		adQueryJobs.Update(jobID, func(job *adQueryJob) {
			job.Status = "failed"
			job.Progress = 100
			job.Error = fmt.Sprintf("failed to connect to Active Directory: %v", err)
			job.UpdatedAt = time.Now().UTC()
		})
		return
	}
	defer client.Close()

	adQueryJobs.Update(jobID, func(job *adQueryJob) {
		if job.CancelRequested {
			job.Status = "cancelled"
			job.Progress = 100
			job.Error = "job cancelled"
			job.UpdatedAt = time.Now().UTC()
			return
		}
		job.Progress = 40
		job.UpdatedAt = time.Now().UTC()
	})

	if job, ok := adQueryJobs.Get(jobID); ok && job.Status == "cancelled" {
		return
	}

	users, err := client.SearchUsers(request.Query, request.Limit)
	if err != nil {
		adQueryJobs.Update(jobID, func(job *adQueryJob) {
			job.Status = "failed"
			job.Progress = 100
			job.Error = fmt.Sprintf("failed to query Active Directory users: %v", err)
			job.UpdatedAt = time.Now().UTC()
		})
		return
	}

	adQueryJobs.Update(jobID, func(job *adQueryJob) {
		if job.CancelRequested {
			job.Status = "cancelled"
			job.Progress = 100
			job.Error = "job cancelled"
			job.UpdatedAt = time.Now().UTC()
			return
		}
		job.Status = "completed"
		job.Progress = 100
		job.Users = users
		job.TotalUsers = len(users)
		job.UpdatedAt = time.Now().UTC()
	})
}

func (application *application) handleListADJobs(context *gin.Context) {
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
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	statusFilter := strings.ToLower(strings.TrimSpace(context.Query("status")))

	items := adQueryJobs.List()
	filtered := make([]adQueryJob, 0, len(items))
	for _, item := range items {
		if statusFilter != "" && strings.ToLower(item.Status) != statusFilter {
			continue
		}
		filtered = append(filtered, item)
	}

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	context.JSON(http.StatusOK, gin.H{
		"items": filtered[start:end],
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

func (application *application) handleGetADJob(context *gin.Context) {
	jobID := strings.TrimSpace(context.Param("id"))
	if jobID == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "job id is required"})
		return
	}

	job, ok := adQueryJobs.Get(jobID)
	if !ok {
		context.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	context.JSON(http.StatusOK, job)
}

func (application *application) handleCancelADJob(context *gin.Context) {
	jobID := strings.TrimSpace(context.Param("id"))
	if jobID == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "job id is required"})
		return
	}

	job, ok := adQueryJobs.Get(jobID)
	if !ok {
		context.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	if job.Status == "completed" || job.Status == "failed" || job.Status == "cancelled" {
		context.JSON(http.StatusConflict, gin.H{"error": "job cannot be cancelled in current state"})
		return
	}

	adQueryJobs.Update(jobID, func(current *adQueryJob) {
		current.CancelRequested = true
		if current.Status == "queued" {
			current.Status = "cancelled"
			current.Progress = 100
			current.Error = "job cancelled"
		}
		current.UpdatedAt = time.Now().UTC()
	})

	updated, _ := adQueryJobs.Get(jobID)
	context.JSON(http.StatusOK, gin.H{
		"job_id": jobID,
		"status": updated.Status,
	})
}

func (application *application) handleRetryADJob(context *gin.Context) {
	jobID := strings.TrimSpace(context.Param("id"))
	if jobID == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "job id is required"})
		return
	}

	job, ok := adQueryJobs.Get(jobID)
	if !ok {
		context.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	if job.Status != "failed" && job.Status != "cancelled" {
		context.JSON(http.StatusConflict, gin.H{"error": "only failed or cancelled jobs can be retried"})
		return
	}

	newJobID := fmt.Sprintf("adjob-%d-%d", time.Now().UTC().UnixNano(), atomic.AddUint64(&adJobSequence, 1))
	now := time.Now().UTC()
	request := job.Request
	adQueryJobs.Set(adQueryJob{
		ID:        newJobID,
		Status:    "queued",
		Progress:  0,
		Query:     strings.TrimSpace(request.Query),
		CreatedAt: now,
		UpdatedAt: now,
		Request:   request,
	})

	go application.runADUserQueryJob(newJobID, request)

	context.JSON(http.StatusAccepted, gin.H{
		"job_id":  newJobID,
		"status":  "queued",
		"message": "retry job created",
	})
}

func (application *application) handleADGroupQuery(context *gin.Context) {
	var request ADGroupQueryRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := request.resolveInto(); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client, err := application.ad.NewGroupClient(request.Server, request.BaseDN, request.Username, request.Password)
	if err != nil {
		context.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to connect to Active Directory: %v", err)})
		return
	}
	defer client.Close()

	groups, err := client.SearchGroups(request.Query, request.Limit)
	if err != nil {
		context.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to query Active Directory groups: %v", err)})
		return
	}

	context.JSON(http.StatusOK, gin.H{
		"query":  strings.TrimSpace(request.Query),
		"count":  len(groups),
		"groups": groups,
	})
}

func (application *application) handleADGroupMembers(context *gin.Context) {
	var request ADGroupMembersRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := request.resolveInto(); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client, err := application.ad.NewGroupClient(request.Server, request.BaseDN, request.Username, request.Password)
	if err != nil {
		context.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to connect to Active Directory: %v", err)})
		return
	}
	defer client.Close()

	group, err := client.GetGroup(context.Request.Context(), request.GroupDN)
	if err != nil {
		context.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to query Active Directory group members: %v", err)})
		return
	}

	response := gin.H{
		"group": group,
	}

	if request.IncludeNested {
		resolverOptions := make([]ad.GroupResolverOption, 0, 2)
		if request.MaxDepth > 0 {
			resolverOptions = append(resolverOptions, ad.WithMaxDepth(request.MaxDepth))
		}

		if len(request.ExcludeGroupPatterns) > 0 || len(request.ExcludeUserPatterns) > 0 {
			filter := ad.NewExclusionFilter()
			for _, pattern := range request.ExcludeGroupPatterns {
				trimmedPattern := strings.TrimSpace(pattern)
				if trimmedPattern != "" {
					filter.AddGroupPattern(trimmedPattern)
				}
			}
			for _, pattern := range request.ExcludeUserPatterns {
				trimmedPattern := strings.TrimSpace(pattern)
				if trimmedPattern != "" {
					filter.AddUserPattern(trimmedPattern)
				}
			}
			resolverOptions = append(resolverOptions, ad.WithExclusionFilter(filter))
		}

		resolver := ad.NewGroupResolver(client, resolverOptions...)
		resolution, err := resolver.ResolveGroupMembers(context.Request.Context(), request.GroupDN)
		if err != nil {
			context.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to resolve nested group members: %v", err)})
			return
		}

		response["resolution"] = resolution
	}

	context.JSON(http.StatusOK, response)
}

func (application *application) handleADPrincipalExpand(context *gin.Context) {
	var request ADPrincipalExpandRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := request.resolveInto(); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	identifiers := dedupeNonEmptyStrings(request.Identifiers)
	if len(identifiers) == 0 {
		context.JSON(http.StatusBadRequest, gin.H{"error": "at least one identifier is required"})
		return
	}

	client, err := application.ad.NewGroupClient(request.Server, request.BaseDN, request.Username, request.Password)
	if err != nil {
		context.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to connect to Active Directory: %v", err)})
		return
	}
	defer client.Close()

	resolverOptions := make([]ad.GroupResolverOption, 0, 2)
	if request.MaxDepth > 0 {
		resolverOptions = append(resolverOptions, ad.WithMaxDepth(request.MaxDepth))
	}

	if len(request.ExcludeGroupPatterns) > 0 || len(request.ExcludeUserPatterns) > 0 {
		filter := ad.NewExclusionFilter()
		for _, pattern := range request.ExcludeGroupPatterns {
			trimmedPattern := strings.TrimSpace(pattern)
			if trimmedPattern != "" {
				filter.AddGroupPattern(trimmedPattern)
			}
		}
		for _, pattern := range request.ExcludeUserPatterns {
			trimmedPattern := strings.TrimSpace(pattern)
			if trimmedPattern != "" {
				filter.AddUserPattern(trimmedPattern)
			}
		}
		resolverOptions = append(resolverOptions, ad.WithExclusionFilter(filter))
	}

	resolver := ad.NewGroupResolver(client, resolverOptions...)
	items := make(map[string]gin.H, len(identifiers))
	warnings := make([]string, 0)

	for _, identifier := range identifiers {
		entry := gin.H{
			"identifier": identifier,
		}

		principal, err := client.ResolvePrincipal(context.Request.Context(), identifier)
		if err != nil {
			entry["error"] = fmt.Sprintf("failed to resolve principal: %v", err)
			warnings = append(warnings, fmt.Sprintf("%s: %v", identifier, err))
			items[strings.ToLower(strings.TrimSpace(identifier))] = entry
			continue
		}

		if principal == nil {
			items[strings.ToLower(strings.TrimSpace(identifier))] = entry
			continue
		}

		entry["principal"] = principal
		if request.IncludeNested && principal.Type == models.ADObjectTypeGroup && strings.TrimSpace(principal.DN) != "" {
			resolution, err := resolver.ResolveGroupMembers(context.Request.Context(), principal.DN)
			if err != nil {
				entry["error"] = fmt.Sprintf("failed to resolve group members: %v", err)
				warnings = append(warnings, fmt.Sprintf("%s members: %v", identifier, err))
			} else {
				entry["resolution"] = resolution
			}
		}

		items[strings.ToLower(strings.TrimSpace(identifier))] = entry
	}

	response := gin.H{
		"count": len(items),
		"items": items,
	}
	if len(warnings) > 0 {
		response["warnings"] = warnings
	}

	context.JSON(http.StatusOK, response)
}

func (application *application) handleADTree(context *gin.Context) {
	var request ADTreeRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := request.resolveInto(); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	clientKey := strings.TrimSpace(context.ClientIP())
	if !adTreeLimiter.Allow(clientKey) {
		context.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded for AD tree requests"})
		return
	}

	cacheKey := buildADTreeCacheKey(request)
	if cached, ok := adTreeCache.Get(cacheKey); ok {
		context.JSON(http.StatusOK, cached)
		return
	}

	client, err := application.ad.NewTreeClient(request.Server, request.BaseDN, request.Username, request.Password)
	if err != nil {
		context.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to connect to Active Directory: %v", err)})
		return
	}
	defer client.Close()

	listing, err := client.ListTreeNodes(context.Request.Context(), request.ParentDN, request.Limit)
	if err != nil {
		message := fmt.Sprintf("failed to query Active Directory tree: %v", err)
		if strings.Contains(strings.ToLower(err.Error()), "size limit exceeded") {
			message = "failed to query Active Directory tree: container is too large for the current directory limits; narrow the scope or use user search on the right"
		}
		context.JSON(http.StatusBadGateway, gin.H{"error": message})
		return
	}

	nodes := listing.Nodes
	filteredNodes := filterADTreeNodes(nodes, request.NodeTypes)
	page, pageSize := normalizeADTreePagination(request.Page, request.PageSize)
	pagedNodes, pagination := paginateADTreeNodes(filteredNodes, page, pageSize)

	parentDN := strings.TrimSpace(request.ParentDN)
	if parentDN == "" {
		parentDN = strings.TrimSpace(request.BaseDN)
	}

	response := gin.H{
		"parent_dn":   parentDN,
		"count":       len(pagedNodes),
		"total_count": len(filteredNodes),
		"nodes":       pagedNodes,
		"pagination":  pagination,
		"partial":     listing.Partial,
		"warning":     listing.Warning,
	}
	adTreeCache.Set(cacheKey, response)
	context.JSON(http.StatusOK, response)
}

func (application *application) handleADTreeExplain(context *gin.Context) {
	var request ADTreeExplainRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	normalizedDN := strings.TrimSpace(request.DN)
	if normalizedDN == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "dn is required"})
		return
	}

	nodeType := strings.ToLower(strings.TrimSpace(request.NodeType))
	if nodeType == "" {
		nodeType = inferNodeTypeFromDN(normalizedDN)
	}

	chain := buildDNInheritanceChain(normalizedDN)
	scopeTargets := buildADScopeTargets(chain, nodeType)
	riskLevel := classifyADInheritanceRisk(nodeType, len(chain))
	recommendedChecks := buildADRecommendedChecks(nodeType, riskLevel)
	ancestryTypes := buildADAncestryTypes(chain)
	policyHints := buildADPolicyHints(chain, nodeType)
	delegationBoundaries := buildADDelegationBoundaries(chain)
	complexityScore := estimateADComplexityScore(nodeType, len(chain), len(policyHints))
	notes := []string{
		"Inheritance chain is derived from distinguished name hierarchy.",
		"Effective permissions still depend on ACL entries and AD group expansion.",
	}
	switch nodeType {
	case "ou":
		notes = append(notes, "OU nodes often inherit delegated permissions from parent OUs.")
	case "policy":
		notes = append(notes, "Policy objects can affect child OUs and computer/user objects through GPO links.")
	case "group":
		notes = append(notes, "Group effective permissions depend on nested membership and deny precedence.")
	case "user":
		notes = append(notes, "User effective permissions combine direct ACL + group ACL + inherited ACL.")
	}

	context.JSON(http.StatusOK, gin.H{
		"dn":                    normalizedDN,
		"node_type":             nodeType,
		"inheritance_chain":     chain,
		"node_name":             extractNodeName(normalizedDN),
		"notes":                 notes,
		"depth":                 len(chain),
		"scope_targets":         scopeTargets,
		"risk_level":            riskLevel,
		"recommended_checks":    recommendedChecks,
		"ancestry_types":        ancestryTypes,
		"policy_hints":          policyHints,
		"delegation_boundaries": delegationBoundaries,
		"complexity_score":      complexityScore,
	})
}

func normalizeADTreePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	return page, pageSize
}

func filterADTreeNodes(nodes []models.ADTreeNode, requestedTypes []string) []models.ADTreeNode {
	if len(requestedTypes) == 0 {
		return append([]models.ADTreeNode(nil), nodes...)
	}

	allowed := make(map[string]struct{}, len(requestedTypes))
	for _, requested := range requestedTypes {
		trimmed := strings.ToLower(strings.TrimSpace(requested))
		if trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return append([]models.ADTreeNode(nil), nodes...)
	}

	filtered := make([]models.ADTreeNode, 0, len(nodes))
	for _, node := range nodes {
		if _, ok := allowed[strings.ToLower(strings.TrimSpace(node.NodeType))]; ok {
			filtered = append(filtered, node)
		}
	}

	return filtered
}

func paginateADTreeNodes(nodes []models.ADTreeNode, page, pageSize int) ([]models.ADTreeNode, gin.H) {
	total := len(nodes)
	start := (page - 1) * pageSize
	if start >= total {
		return []models.ADTreeNode{}, gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": 0,
		}
	}

	end := start + pageSize
	if end > total {
		end = total
	}
	totalPages := (total + pageSize - 1) / pageSize
	return nodes[start:end], gin.H{
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	}
}

type adTreeResponseCache struct {
	items map[string]cachedADTreeResponse
	mutex sync.RWMutex
	ttl   time.Duration
}

type cachedADTreeResponse struct {
	value      gin.H
	expireTime time.Time
}

func newADTreeResponseCache(ttl time.Duration) *adTreeResponseCache {
	if ttl < time.Second {
		ttl = 10 * time.Second
	}

	return &adTreeResponseCache{
		items: make(map[string]cachedADTreeResponse),
		ttl:   ttl,
	}
}

func (cache *adTreeResponseCache) Get(key string) (gin.H, bool) {
	cache.mutex.RLock()
	entry, ok := cache.items[key]
	cache.mutex.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().UTC().After(entry.expireTime) {
		cache.mutex.Lock()
		delete(cache.items, key)
		cache.mutex.Unlock()
		return nil, false
	}

	return entry.value, true
}

func (cache *adTreeResponseCache) Set(key string, value gin.H) {
	cache.mutex.Lock()
	cache.items[key] = cachedADTreeResponse{
		value:      value,
		expireTime: time.Now().UTC().Add(cache.ttl),
	}
	cache.mutex.Unlock()
}

func buildADTreeCacheKey(request ADTreeRequest) string {
	nodeTypes := make([]string, 0, len(request.NodeTypes))
	for _, nodeType := range request.NodeTypes {
		trimmed := strings.ToLower(strings.TrimSpace(nodeType))
		if trimmed != "" {
			nodeTypes = append(nodeTypes, trimmed)
		}
	}
	sort.Strings(nodeTypes)

	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(request.Server)),
		strings.ToLower(strings.TrimSpace(request.BaseDN)),
		strings.ToLower(strings.TrimSpace(request.Username)),
		strings.ToLower(strings.TrimSpace(request.ParentDN)),
		strconv.Itoa(request.Limit),
		strconv.Itoa(request.Page),
		strconv.Itoa(request.PageSize),
		strings.Join(nodeTypes, "|"),
	}, "::")
}

func buildDNInheritanceChain(distinguishedName string) []string {
	parts := strings.Split(strings.TrimSpace(distinguishedName), ",")
	chain := make([]string, 0, len(parts))
	for index := 0; index < len(parts); index++ {
		chain = append(chain, strings.Join(parts[index:], ","))
	}

	return chain
}

func inferNodeTypeFromDN(distinguishedName string) string {
	upper := strings.ToUpper(strings.TrimSpace(distinguishedName))
	switch {
	case strings.HasPrefix(upper, "OU="):
		return "ou"
	case strings.HasPrefix(upper, "CN={") || strings.Contains(upper, "CN=POLICIES"):
		return "policy"
	case strings.HasPrefix(upper, "CN="):
		return "container"
	case strings.HasPrefix(upper, "DC="):
		return "domain"
	default:
		return "unknown"
	}
}

func extractNodeName(distinguishedName string) string {
	firstPart := strings.Split(strings.TrimSpace(distinguishedName), ",")[0]
	if index := strings.Index(firstPart, "="); index >= 0 && index+1 < len(firstPart) {
		return strings.TrimSpace(firstPart[index+1:])
	}

	return firstPart
}

func buildADScopeTargets(chain []string, nodeType string) []string {
	if len(chain) == 0 {
		return []string{}
	}

	targets := make([]string, 0, len(chain)+1)
	switch nodeType {
	case "policy":
		targets = append(targets, "Linked GPO target OUs and domain root")
	case "group":
		targets = append(targets, "Nested group memberships and ACL bindings")
	case "user":
		targets = append(targets, "Direct ACL assignments and inherited group ACL")
	default:
		targets = append(targets, "Parent containers in AD hierarchy")
	}

	limit := len(chain)
	if limit > 5 {
		limit = 5
	}
	for index := 0; index < limit; index++ {
		targets = append(targets, chain[index])
	}

	return targets
}

func classifyADInheritanceRisk(nodeType string, depth int) string {
	if nodeType == "policy" || nodeType == "group" {
		return "high"
	}
	if depth >= 5 {
		return "high"
	}
	if depth >= 3 {
		return "medium"
	}
	return "low"
}

func buildADRecommendedChecks(nodeType, riskLevel string) []string {
	checks := []string{
		"Validate explicit deny entries and delegated ACLs at parent containers.",
		"Review nested group expansion used for effective permission calculation.",
	}
	switch nodeType {
	case "policy":
		checks = append(checks, "Verify GPO links, security filtering, and inheritance blocking.")
	case "group":
		checks = append(checks, "Audit nested groups and privileged group membership drift.")
	case "ou":
		checks = append(checks, "Confirm OU-level delegation boundaries and inheritance expectations.")
	}
	if riskLevel == "high" {
		checks = append(checks, "Prioritize this node in remediation backlog and re-scan after changes.")
	}

	return checks
}

func buildADAncestryTypes(chain []string) []string {
	types := make([]string, 0, len(chain))
	for _, item := range chain {
		types = append(types, inferNodeTypeFromDN(item))
	}
	return types
}

func buildADPolicyHints(chain []string, nodeType string) []string {
	hints := make([]string, 0, 4)
	if nodeType == "policy" {
		hints = append(hints, "Policy object may override child OU settings via GPO links.")
	}
	for _, item := range chain {
		upper := strings.ToUpper(item)
		if strings.Contains(upper, "CN=POLICIES") {
			hints = append(hints, "Hierarchy includes CN=Policies container.")
			break
		}
	}
	if len(chain) >= 4 {
		hints = append(hints, "Deep hierarchy can increase inherited policy complexity.")
	}
	if len(hints) == 0 {
		hints = append(hints, "No explicit policy container found in current inheritance chain.")
	}
	return hints
}

func buildADDelegationBoundaries(chain []string) []string {
	boundaries := make([]string, 0, len(chain))
	for _, item := range chain {
		nodeType := inferNodeTypeFromDN(item)
		if nodeType == "ou" || nodeType == "domain" {
			boundaries = append(boundaries, item)
		}
	}
	if len(boundaries) == 0 && len(chain) > 0 {
		boundaries = append(boundaries, chain[len(chain)-1])
	}
	if len(boundaries) > 5 {
		boundaries = boundaries[:5]
	}
	return boundaries
}

func estimateADComplexityScore(nodeType string, depth int, policyHintCount int) int {
	score := depth * 10
	switch nodeType {
	case "policy":
		score += 35
	case "group":
		score += 25
	case "ou":
		score += 20
	case "user":
		score += 10
	default:
		score += 5
	}
	score += policyHintCount * 8
	if score > 100 {
		return 100
	}
	if score < 1 {
		return 1
	}
	return score
}

type adQueryJob struct {
	ID              string                  `json:"id"`
	Status          string                  `json:"status"`
	Progress        int                     `json:"progress"`
	Query           string                  `json:"query"`
	TotalUsers      int                     `json:"total_users"`
	Users           []ad.User               `json:"users,omitempty"`
	Error           string                  `json:"error,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
	Request         ADUserAsyncQueryRequest `json:"-"`
	CancelRequested bool                    `json:"-"`
}

type adQueryJobStore struct {
	mutex sync.RWMutex
	limit int
	jobs  map[string]adQueryJob
	order []string
}

func newADQueryJobStore(limit int) *adQueryJobStore {
	if limit < 10 {
		limit = 100
	}

	return &adQueryJobStore{
		limit: limit,
		jobs:  make(map[string]adQueryJob),
		order: make([]string, 0, limit),
	}
}

func (store *adQueryJobStore) Set(job adQueryJob) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	if _, exists := store.jobs[job.ID]; !exists {
		store.order = append(store.order, job.ID)
	}
	store.jobs[job.ID] = job

	for len(store.order) > store.limit {
		oldestID := store.order[0]
		store.order = store.order[1:]
		delete(store.jobs, oldestID)
	}
}

func (store *adQueryJobStore) Update(id string, update func(job *adQueryJob)) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	job, ok := store.jobs[id]
	if !ok {
		return
	}

	update(&job)
	store.jobs[id] = job
}

func (store *adQueryJobStore) Get(id string) (adQueryJob, bool) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	job, ok := store.jobs[id]
	return job, ok
}

func (store *adQueryJobStore) List() []adQueryJob {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	items := make([]adQueryJob, 0, len(store.order))
	for index := len(store.order) - 1; index >= 0; index-- {
		jobID := store.order[index]
		if job, ok := store.jobs[jobID]; ok {
			items = append(items, job)
		}
	}

	return items
}

func (application *application) handleADTest(context *gin.Context) {
	var request ADConnectionRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := request.resolveInto(); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client := application.ad.NewConnectionClient(request.Server, request.BaseDN, request.Username, request.Password)
	if err := client.Connect(); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "AD connection failed: " + err.Error()})
		return
	}
	defer client.Close()

	context.JSON(http.StatusOK, gin.H{"message": "Active Directory connection successful"})
}
