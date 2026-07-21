package main

import (
	"errors"
	"net/http"

	"github.com/weibinliao/OpenAD/internal/access"
	"github.com/weibinliao/OpenAD/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type accessByUserRequest struct {
	Principal  string   `json:"principal" binding:"required"`
	SyncRunID  string   `json:"sync_run_id"`
	SessionIDs []string `json:"session_ids"`
}

type accessByResourceRequest struct {
	PathPrefix string `json:"path_prefix" binding:"required"`
	SessionID  string `json:"session_id"`
	SyncRunID  string `json:"sync_run_id"`
}

// accessService returns the cross-analysis access engine when persistence is
// available; endpoints degrade with 503 otherwise (same pattern as adsync).
func accessService() *access.Service {
	if !database.Ready() {
		return nil
	}
	return access.NewService(database.DB)
}

func respondAccessUnavailable(context *gin.Context) {
	context.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not initialized; access analysis is unavailable"})
}

func respondAccessError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, access.ErrPrincipalRequired), errors.Is(err, access.ErrPathRequired):
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, access.ErrPrincipalNotFound):
		context.JSON(http.StatusNotFound, gin.H{"error": "principal not found in the directory snapshot; check the spelling or run a new directory sync"})
	case errors.Is(err, access.ErrSyncRunNotFound):
		context.JSON(http.StatusNotFound, gin.H{"error": "directory sync run not found"})
	case errors.Is(err, access.ErrSessionNotFound):
		context.JSON(http.StatusNotFound, gin.H{"error": "scan session not found"})
	case errors.Is(err, access.ErrNoCompletedSyncRun):
		context.JSON(http.StatusConflict, gin.H{"error": "no completed directory sync snapshot exists yet; run a directory sync first (Settings > Directory Sync)"})
	case errors.Is(err, access.ErrNoScanSessions):
		context.JSON(http.StatusConflict, gin.H{"error": "no completed scan sessions exist yet; run a folder scan first"})
	case errors.Is(err, access.ErrNoMatchingSession):
		context.JSON(http.StatusConflict, gin.H{"error": "no completed scan session covers this path; scan the folder first or pass an explicit session_id"})
	default:
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (application *application) handleAccessByUser(context *gin.Context) {
	service := accessService()
	if service == nil {
		respondAccessUnavailable(context)
		return
	}

	var request accessByUserRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := access.ByUserInput{Principal: request.Principal}
	if request.SyncRunID != "" {
		runID, err := uuid.Parse(request.SyncRunID)
		if err != nil {
			context.JSON(http.StatusBadRequest, gin.H{"error": "invalid sync_run_id"})
			return
		}
		input.SyncRunID = runID
	}
	for _, rawID := range request.SessionIDs {
		sessionID, err := uuid.Parse(rawID)
		if err != nil {
			context.JSON(http.StatusBadRequest, gin.H{"error": "invalid session id: " + rawID})
			return
		}
		input.SessionIDs = append(input.SessionIDs, sessionID)
	}

	result, err := service.ByUser(input)
	if err != nil {
		respondAccessError(context, err)
		return
	}

	context.JSON(http.StatusOK, result)
}

func (application *application) handleAccessByResource(context *gin.Context) {
	service := accessService()
	if service == nil {
		respondAccessUnavailable(context)
		return
	}

	var request accessByResourceRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := access.ByResourceInput{PathPrefix: request.PathPrefix}
	if request.SessionID != "" {
		sessionID, err := uuid.Parse(request.SessionID)
		if err != nil {
			context.JSON(http.StatusBadRequest, gin.H{"error": "invalid session_id"})
			return
		}
		input.SessionID = sessionID
	}
	if request.SyncRunID != "" {
		runID, err := uuid.Parse(request.SyncRunID)
		if err != nil {
			context.JSON(http.StatusBadRequest, gin.H{"error": "invalid sync_run_id"})
			return
		}
		input.SyncRunID = runID
	}

	result, err := service.ByResource(input)
	if err != nil {
		respondAccessError(context, err)
		return
	}

	context.JSON(http.StatusOK, result)
}
