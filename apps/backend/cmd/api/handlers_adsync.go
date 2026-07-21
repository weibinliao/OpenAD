package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/weibinliao/OpenAD/internal/ad"
	"github.com/weibinliao/OpenAD/internal/adsync"
	"github.com/weibinliao/OpenAD/internal/connections"
	"github.com/weibinliao/OpenAD/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// adSyncStartMutex serializes the running-run check with run creation so two
// concurrent POSTs cannot both pass the single-active-run guard.
var adSyncStartMutex sync.Mutex

type directorySyncRequest struct {
	ConnectionID string `json:"connection_id" binding:"required"`
}

// adSyncService returns the directory-sync service when persistence is
// available; endpoints degrade with 503 otherwise.
func adSyncService() *adsync.Service {
	if !database.Ready() {
		return nil
	}
	return adsync.NewService(database.DB)
}

func respondADSyncUnavailable(context *gin.Context) {
	context.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not initialized; directory sync is unavailable"})
}

func (application *application) handleStartDirectorySync(context *gin.Context) {
	service := adSyncService()
	connectionService := connectionsService()
	if service == nil || connectionService == nil {
		respondADSyncUnavailable(context)
		return
	}

	var request directorySyncRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	connectionID, err := uuid.Parse(request.ConnectionID)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid connection_id"})
		return
	}

	credentials, err := connectionService.Resolve(connectionID)
	if err != nil {
		if errors.Is(err, connections.ErrNotFound) {
			context.JSON(http.StatusNotFound, gin.H{"error": "connection profile not found"})
			return
		}
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	adSyncStartMutex.Lock()
	defer adSyncStartMutex.Unlock()

	running, err := service.HasRunningRun()
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if running {
		context.JSON(http.StatusConflict, gin.H{"error": "another directory sync run is already in progress"})
		return
	}

	run, err := service.StartRun(connectionID)
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	go runDirectorySync(service, run.ID, credentials)

	context.JSON(http.StatusAccepted, gin.H{
		"run_id":  run.ID,
		"status":  run.Status,
		"message": "directory sync started",
	})
}

func runDirectorySync(service *adsync.Service, runID uuid.UUID, credentials *connections.ResolvedCredentials) {
	client, err := ad.NewADClient(credentials.Server, credentials.BaseDN, credentials.BindUser, credentials.Password)
	if err != nil {
		service.MarkRunFailed(runID, fmt.Sprintf("failed to connect to Active Directory: %v", err))
		return
	}
	defer client.Close()

	// Run updates the row to completed/failed itself; the error is already
	// recorded, and there is no request left to report it to.
	_ = service.Run(context.Background(), runID, client)
}

func (application *application) handleListDirectorySyncRuns(context *gin.Context) {
	service := adSyncService()
	if service == nil {
		respondADSyncUnavailable(context)
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
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	runs, total, err := service.ListRuns(page, pageSize)
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	context.JSON(http.StatusOK, gin.H{
		"items": runs,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

func (application *application) handleGetDirectorySyncRun(context *gin.Context) {
	service := adSyncService()
	if service == nil {
		respondADSyncUnavailable(context)
		return
	}

	id, err := uuid.Parse(context.Param("id"))
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid run id"})
		return
	}

	run, err := service.GetRun(id)
	if err != nil {
		if errors.Is(err, adsync.ErrRunNotFound) {
			context.JSON(http.StatusNotFound, gin.H{"error": "directory sync run not found"})
			return
		}
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	context.JSON(http.StatusOK, run)
}
