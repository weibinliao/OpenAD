package main

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/weibinliao/OpenAD/internal/scanservice"
	"github.com/gin-gonic/gin"
)

type scanCancelRegistry struct {
	mutex   sync.Mutex
	cancels map[string]context.CancelFunc
}

func newScanCancelRegistry() *scanCancelRegistry {
	return &scanCancelRegistry{
		cancels: make(map[string]context.CancelFunc),
	}
}

func (registry *scanCancelRegistry) register(scanID string, cancel context.CancelFunc) {
	scanID = strings.TrimSpace(scanID)
	if scanID == "" || cancel == nil {
		return
	}

	registry.mutex.Lock()
	registry.cancels[scanID] = cancel
	registry.mutex.Unlock()
}

func (registry *scanCancelRegistry) cancel(scanID string) bool {
	scanID = strings.TrimSpace(scanID)
	if scanID == "" {
		return false
	}

	registry.mutex.Lock()
	cancel, ok := registry.cancels[scanID]
	if ok {
		delete(registry.cancels, scanID)
	}
	registry.mutex.Unlock()

	if !ok {
		return false
	}

	cancel()
	return true
}

func (registry *scanCancelRegistry) remove(scanID string) {
	scanID = strings.TrimSpace(scanID)
	if scanID == "" {
		return
	}

	registry.mutex.Lock()
	delete(registry.cancels, scanID)
	registry.mutex.Unlock()
}

func (application *application) handleCancelScan(context *gin.Context) {
	scanID := strings.TrimSpace(context.Param("id"))
	if scanID == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "scan id is required"})
		return
	}

	cancelled := application.scanCancels.cancel(scanID)
	if !cancelled {
		context.JSON(http.StatusNotFound, gin.H{"error": "scan is not active"})
		return
	}

	application.progressHub.publish(scanID, scanservice.ProgressEvent{
		ScanID: scanID,
		Status: "cancelled",
		Error:  "scan cancelled",
	})

	context.JSON(http.StatusOK, gin.H{
		"scan_id": scanID,
		"status":  "cancelled",
		"message": "scan cancellation requested",
	})
}
