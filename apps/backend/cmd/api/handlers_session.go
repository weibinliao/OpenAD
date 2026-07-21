package main

import (
	"net/http"

	"github.com/weibinliao/OpenAD/internal/comparisonservice"
	"github.com/weibinliao/OpenAD/internal/historyservice"
	"github.com/gin-gonic/gin"
)

type CompareRequest struct {
	BaselineSessionID string `json:"baseline_session_id" binding:"required"`
	CurrentSessionID  string `json:"current_session_id" binding:"required"`
}

func (application *application) handleCompare(context *gin.Context) {
	var request CompareRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	report, err := application.comparison.Compare(comparisonservice.Request{
		BaselineSessionID: request.BaselineSessionID,
		CurrentSessionID:  request.CurrentSessionID,
	})
	if err != nil {
		handleHistoryError(context, err)
		return
	}

	context.JSON(http.StatusOK, report)
}

func (application *application) handleListSessions(context *gin.Context) {
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

	response, err := application.history.ListSessions(historyservice.SessionListFilter{
		Page:     page,
		PageSize: pageSize,
		Status:   context.Query("status"),
		RootPath: context.Query("root_path"),
	})
	if err != nil {
		handleHistoryError(context, err)
		return
	}

	context.JSON(http.StatusOK, response)
}

func (application *application) handleGetSession(context *gin.Context) {
	response, err := application.history.GetSession(context.Param("id"))
	if err != nil {
		handleHistoryError(context, err)
		return
	}

	context.JSON(http.StatusOK, response)
}

func (application *application) handleGetSessionBundle(context *gin.Context) {
	response, err := application.history.GetSessionBundle(context.Param("id"))
	if err != nil {
		handleHistoryError(context, err)
		return
	}

	context.JSON(http.StatusOK, response)
}

func (application *application) handleListSessionPermissions(context *gin.Context) {
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

	inherited, err := parseBoolQuery(context, "inherited")
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := application.history.ListSessionPermissions(context.Param("id"), historyservice.PermissionListFilter{
		Page:      page,
		PageSize:  pageSize,
		Path:      context.Query("path"),
		Trustee:   context.Query("trustee"),
		Inherited: inherited,
	})
	if err != nil {
		handleHistoryError(context, err)
		return
	}

	context.JSON(http.StatusOK, response)
}

func (application *application) handleListSessionChanges(context *gin.Context) {
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

	response, err := application.history.ListSessionChanges(context.Param("id"), historyservice.ChangeListFilter{
		Page:       page,
		PageSize:   pageSize,
		ChangeType: context.Query("change_type"),
		Path:       context.Query("path"),
		Trustee:    context.Query("trustee"),
	})
	if err != nil {
		handleHistoryError(context, err)
		return
	}

	context.JSON(http.StatusOK, response)
}
