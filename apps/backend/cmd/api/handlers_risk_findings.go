package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/weibinliao/OpenAD/internal/riskservice"
)

const maxRiskFindingRequestBytes = 32 << 20

type riskFindingBatchRequest struct {
	Items []riskservice.FindingInput `json:"items"`
}

type riskFindingStatusRequest struct {
	Status string  `json:"status"`
	Note   *string `json:"note,omitempty"`
}

func (application *application) handleListRiskFindings(context *gin.Context) {
	items, err := application.riskFindings.List()
	if err != nil {
		handleRiskFindingError(context, err)
		return
	}
	context.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

func (application *application) handleUpsertRiskFindings(context *gin.Context) {
	request, ok := bindRiskFindingBatch(context)
	if !ok {
		return
	}
	count, err := application.riskFindings.UpsertFromScan(request.Items)
	if err != nil {
		handleRiskFindingError(context, err)
		return
	}
	context.JSON(http.StatusOK, gin.H{"count": count})
}

func (application *application) handleImportRiskFindings(context *gin.Context) {
	request, ok := bindRiskFindingBatch(context)
	if !ok {
		return
	}
	count, err := application.riskFindings.ImportLegacy(request.Items)
	if err != nil {
		handleRiskFindingError(context, err)
		return
	}
	context.JSON(http.StatusOK, gin.H{"count": count})
}

func (application *application) handleUpdateRiskFinding(context *gin.Context) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maxRiskFindingRequestBytes)
	var request riskFindingStatusRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		handleRiskFindingBindError(context, err)
		return
	}
	if strings.TrimSpace(request.Status) == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
		return
	}

	finding, err := application.riskFindings.UpdateStatus(context.Param("id"), request.Status, request.Note)
	if err != nil {
		handleRiskFindingError(context, err)
		return
	}
	context.JSON(http.StatusOK, finding)
}

func bindRiskFindingBatch(context *gin.Context) (riskFindingBatchRequest, bool) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maxRiskFindingRequestBytes)
	var request riskFindingBatchRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		handleRiskFindingBindError(context, err)
		return riskFindingBatchRequest{}, false
	}
	return request, true
}

func handleRiskFindingBindError(context *gin.Context, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		context.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "risk finding payload exceeds 32 MiB"})
		return
	}
	context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func handleRiskFindingError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, riskservice.ErrDatabaseUnavailable):
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, riskservice.ErrInvalidInput),
		errors.Is(err, riskservice.ErrInvalidID),
		errors.Is(err, riskservice.ErrInvalidStatus):
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, riskservice.ErrNotFound):
		context.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
