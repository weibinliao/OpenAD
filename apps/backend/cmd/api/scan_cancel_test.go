package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weibinliao/OpenAD/internal/scanservice"
)

func TestScanCancelRegistryRejectsConcurrentDuplicateRegistration(t *testing.T) {
	registry := newScanCancelRegistry()
	const attempts = 32

	type registrationResult struct {
		registration *scanCancelRegistration
		err          error
	}
	results := make(chan registrationResult, attempts)
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	for index := 0; index < attempts; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			registration, err := registry.register("duplicate-scan", func() {})
			results <- registrationResult{registration: registration, err: err}
		}()
	}

	close(start)
	waitGroup.Wait()
	close(results)

	successCount := 0
	conflictCount := 0
	for result := range results {
		switch {
		case result.err == nil:
			successCount++
			require.NotNil(t, result.registration)
		case errors.Is(result.err, errScanIDAlreadyActive):
			conflictCount++
		default:
			t.Fatalf("unexpected registration error: %v", result.err)
		}
	}

	assert.Equal(t, 1, successCount)
	assert.Equal(t, attempts-1, conflictCount)
}

func TestScanCancelRegistryKeepsIDReservedUntilMatchingRegistrationExits(t *testing.T) {
	registry := newScanCancelRegistry()
	var firstCancelCalls atomic.Int32
	var secondCancelCalls atomic.Int32

	first, err := registry.register("reused-scan", func() { firstCancelCalls.Add(1) })
	require.NoError(t, err)
	require.True(t, registry.cancel("reused-scan"))

	registrationWhileStopping, err := registry.register("reused-scan", func() {})
	assert.Nil(t, registrationWhileStopping)
	assert.ErrorIs(t, err, errScanIDAlreadyActive)
	assert.True(t, registry.remove("reused-scan", first))

	second, err := registry.register("reused-scan", func() { secondCancelCalls.Add(1) })
	require.NoError(t, err)

	assert.False(t, registry.remove("reused-scan", first))
	assert.True(t, registry.cancel("reused-scan"))
	assert.Equal(t, int32(1), firstCancelCalls.Load())
	assert.Equal(t, int32(1), secondCancelCalls.Load())
	assert.NotEqual(t, first, second)
}

func TestScanEndpointRejectsDuplicateScanID(t *testing.T) {
	registry := newScanCancelRegistry()
	registration, err := registry.register("duplicate-scan", func() {})
	require.NoError(t, err)
	t.Cleanup(func() { registry.remove("duplicate-scan", registration) })

	var runCalls atomic.Int32
	router := newTestRouter(applicationDependencies{
		scanCancels: registry,
		scans: &stubScanRunner{runFunc: func(request scanservice.Request) (*scanservice.Response, error) {
			runCalls.Add(1)
			return nil, context.Canceled
		}},
	})

	recorder := performJSONRequestToRouter(t, router, "/api/scan", ScanRequest{
		Path:   `C:\Finance`,
		ScanID: "duplicate-scan",
	})

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "scan id is already active")
	assert.Zero(t, runCalls.Load())
}

func TestScanEndpointRejectsWhenScanCapacityIsFull(t *testing.T) {
	directoryService := &stubADService{}
	router := newTestRouter(applicationDependencies{
		scans: &stubScanRunner{runFunc: func(request scanservice.Request) (*scanservice.Response, error) {
			return nil, scanservice.ErrScanConcurrencyLimitReached
		}},
		ad: directoryService,
	})

	recorder := performJSONRequestToRouter(t, router, "/api/scan", ScanRequest{
		Path:   `C:\Finance`,
		ScanID: "capacity-check",
		EffectivePermissions: &EffectivePermissionRequest{
			Enabled:  true,
			Server:   "ldap://directory.example.com",
			BaseDN:   "DC=example,DC=com",
			Username: "svc-reader",
			Password: "secret",
		},
	})

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "maximum concurrent scans reached")
	assert.Zero(t, directoryService.permissionExpanderCalls)
}

func TestScanEndpointRejectsUNCServerRoot(t *testing.T) {
	var runCalls atomic.Int32
	router := newTestRouter(applicationDependencies{
		scans: &stubScanRunner{runFunc: func(request scanservice.Request) (*scanservice.Response, error) {
			runCalls.Add(1)
			return nil, nil
		}},
	})

	recorder := performJSONRequestToRouter(t, router, "/api/scan", ScanRequest{
		Path:   `\\server`,
		ScanID: "server-root-scan",
	})

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "UNC share or subdirectory")
	assert.Zero(t, runCalls.Load())
}

func TestScanEndpointKeepsIDReservedAfterClientDisconnectUntilRunExits(t *testing.T) {
	registry := newScanCancelRegistry()
	started := make(chan struct{})
	release := make(chan struct{})
	runExited := make(chan struct{})
	router := newTestRouter(applicationDependencies{
		scanCancels: registry,
		scans: &stubScanRunner{runFunc: func(request scanservice.Request) (*scanservice.Response, error) {
			close(started)
			<-request.Context.Done()
			<-release
			close(runExited)
			return nil, context.Canceled
		}},
	})

	payload, err := json.Marshal(ScanRequest{Path: `C:\Finance`, ScanID: "disconnecting-scan"})
	require.NoError(t, err)
	requestContext, cancelRequest := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodPost, "/api/scan", bytes.NewReader(payload)).WithContext(requestContext)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handlerDone := make(chan struct{})
	go func() {
		router.ServeHTTP(recorder, request)
		close(handlerDone)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("scan did not start")
	}
	cancelRequest()
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler did not return after the client disconnected")
	}

	conflict := performJSONRequestToRouter(t, router, "/api/scan", ScanRequest{
		Path:   `C:\Finance`,
		ScanID: "disconnecting-scan",
	})
	assert.Equal(t, http.StatusConflict, conflict.Code)
	assert.Contains(t, conflict.Body.String(), "scan id is already active")

	close(release)
	select {
	case <-runExited:
	case <-time.After(time.Second):
		t.Fatal("scan runner did not exit")
	}
	require.Eventually(t, func() bool {
		registration, registerErr := registry.register("disconnecting-scan", func() {})
		if registerErr != nil {
			return false
		}
		return registry.remove("disconnecting-scan", registration)
	}, time.Second, 10*time.Millisecond)
}
