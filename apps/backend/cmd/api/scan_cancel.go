package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/weibinliao/OpenAD/internal/scanservice"
)

type scanCancelRegistry struct {
	mutex   sync.Mutex
	cancels map[string]*scanCancelRegistration
}

type scanCancelRegistration struct {
	cancel          context.CancelFunc
	cancelRequested bool
}

var errScanIDAlreadyActive = errors.New("scan id is already active")

func newScanCancelRegistry() *scanCancelRegistry {
	return &scanCancelRegistry{
		cancels: make(map[string]*scanCancelRegistration),
	}
}

func (registry *scanCancelRegistry) register(scanID string, cancel context.CancelFunc) (*scanCancelRegistration, error) {
	scanID = strings.TrimSpace(scanID)
	if scanID == "" || cancel == nil {
		return nil, nil
	}

	registry.mutex.Lock()
	defer registry.mutex.Unlock()
	if _, exists := registry.cancels[scanID]; exists {
		return nil, errScanIDAlreadyActive
	}

	registration := &scanCancelRegistration{cancel: cancel}
	registry.cancels[scanID] = registration
	return registration, nil
}

func (registry *scanCancelRegistry) cancel(scanID string) bool {
	scanID = strings.TrimSpace(scanID)
	if scanID == "" {
		return false
	}

	registry.mutex.Lock()
	registration, ok := registry.cancels[scanID]
	shouldCancel := ok && !registration.cancelRequested
	if shouldCancel {
		registration.cancelRequested = true
	}
	registry.mutex.Unlock()

	if !ok {
		return false
	}

	if shouldCancel {
		registration.cancel()
	}
	return true
}

func (registry *scanCancelRegistry) remove(scanID string, registration *scanCancelRegistration) bool {
	scanID = strings.TrimSpace(scanID)
	if scanID == "" || registration == nil {
		return false
	}

	registry.mutex.Lock()
	defer registry.mutex.Unlock()
	current, exists := registry.cancels[scanID]
	if !exists || current != registration {
		return false
	}

	delete(registry.cancels, scanID)
	return true
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
