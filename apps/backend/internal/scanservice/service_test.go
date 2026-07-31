package scanservice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/scanner"
)

func TestNewService(t *testing.T) {
	service := New()
	assert.NotNil(t, service)
}

func TestRunReturnsResponseAndEmitsProgress(t *testing.T) {
	stubScanner := &stubDirectoryScanner{
		result: &scanner.Result{
			RootPath:         `C:\Finance`,
			MaxDepth:         3,
			IncludeInherited: true,
			ItemsScanned:     4,
			Permissions: []scanner.Permission{{
				Path:       `C:\Finance`,
				Trustee:    `DOMAIN\Alice`,
				TrusteeSID: "S-1-5-21-100",
				Rights:     "Read",
				Type:       "Allow",
				Inherited:  false,
				Source:     "Explicit",
			}},
			Skipped: []scanner.PathError{{
				Path:  `C:\Finance\Archive`,
				Error: "access denied",
			}},
		},
		progressEvents: []scanner.Progress{{
			ItemsScanned:    2,
			PermissionCount: 1,
			CurrentPath:     `C:\Finance`,
		}},
	}
	repository := &stubSessionRepository{}
	service := NewWithDependencies(stubScanner, repository)

	progressEvents := make([]ProgressEvent, 0, 3)
	response, err := service.Run(Request{
		ScanID:           "scan-123",
		Path:             `C:\Finance`,
		MaxDepth:         3,
		IncludeInherited: true,
		Progress: func(event ProgressEvent) {
			progressEvents = append(progressEvents, event)
		},
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, `C:\Finance`, stubScanner.receivedPath)
	assert.Equal(t, 3, stubScanner.receivedOptions.MaxDepth)
	assert.True(t, stubScanner.receivedOptions.IncludeInherited)
	assert.Equal(t, `C:\Finance`, response.RootPath)
	assert.Equal(t, 4, response.ItemsScanned)
	assert.Equal(t, 1, response.PermissionCount)
	assert.Len(t, response.Permissions, 1)
	assert.Len(t, response.Skipped, 1)
	assert.False(t, response.StartedAt.IsZero())
	assert.False(t, response.FinishedAt.IsZero())

	require.Len(t, progressEvents, 3)
	assert.Equal(t, "scan-123", progressEvents[0].ScanID)
	assert.Equal(t, "running", progressEvents[0].Status)
	assert.Equal(t, `C:\Finance`, progressEvents[0].CurrentPath)
	assert.Equal(t, "running", progressEvents[1].Status)
	assert.Equal(t, 2, progressEvents[1].ItemsScanned)
	assert.Equal(t, `C:\Finance`, progressEvents[1].CurrentPath)
	assert.Equal(t, "completed", progressEvents[2].Status)
	assert.Equal(t, 4, progressEvents[2].ItemsScanned)
	assert.Equal(t, 1, progressEvents[2].PermissionCount)
	assert.Equal(t, `C:\Finance`, progressEvents[2].CurrentPath)
	assert.Equal(t, "scan-123", progressEvents[2].ScanID)
	assert.NotEmpty(t, response.SessionID)
	assert.Equal(t, response.SessionID, progressEvents[2].SessionID)
	assert.Equal(t, 1, repository.createCalls)
	assert.Equal(t, 1, repository.completeCalls)
	assert.Nil(t, repository.failedErr)
	assert.Equal(t, response.ItemsScanned, repository.completedResponse.ItemsScanned)
}

func TestRunPersistsCompletedSessionAndPermissions(t *testing.T) {
	service := NewWithDependencies(&stubDirectoryScanner{
		result: &scanner.Result{
			RootPath:         `C:\Finance`,
			MaxDepth:         2,
			IncludeInherited: false,
			ItemsScanned:     3,
			Permissions: []scanner.Permission{{
				Path:       `C:\Finance`,
				Trustee:    `DOMAIN\Alice`,
				TrusteeSID: "S-1-5-21-100",
				Rights:     "Read",
				Type:       "Allow",
				Inherited:  false,
				Source:     "Explicit",
			}},
		},
	}, &stubSessionRepository{})

	response, err := service.Run(Request{
		Path:             `C:\Finance`,
		MaxDepth:         2,
		IncludeInherited: false,
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	require.NotEmpty(t, response.SessionID)
	require.NotNil(t, service.repository)

	repository := service.repository.(*stubSessionRepository)
	require.NotNil(t, repository.createdSession)
	require.NotNil(t, repository.completedSession)
	assert.Equal(t, `C:\Finance`, repository.createdSession.RootPath)
	assert.Equal(t, "running", repository.createdSession.Status)
	assert.Equal(t, "completed", repository.completedSession.Status)
	assert.Equal(t, 3, repository.completedSession.ItemsScanned)
	assert.Equal(t, 1, repository.completedSession.PermissionCount)
	require.NotNil(t, repository.completedResponse)
	assert.Len(t, repository.completedResponse.Permissions, 1)
	assert.Equal(t, `DOMAIN\Alice`, repository.completedResponse.Permissions[0].Trustee)
}

func TestRunExpandsEffectivePermissionsWhenConfigured(t *testing.T) {
	stubScanner := &stubDirectoryScanner{
		result: &scanner.Result{
			RootPath:         `C:\Finance`,
			MaxDepth:         2,
			IncludeInherited: true,
			ItemsScanned:     1,
			Permissions: []scanner.Permission{{
				Path:       `C:\Finance`,
				Trustee:    `DOMAIN\Finance`,
				TrusteeSID: "S-1-5-21-group",
				Rights:     "Read",
				Type:       "Allow",
				Source:     "Explicit",
			}},
		},
	}
	expander := &stubEffectivePermissionExpander{
		expanded: []scanner.Permission{{
			Path:                      `C:\Finance`,
			Trustee:                   `alice`,
			TrusteeSID:                "S-1-5-21-user",
			Rights:                    "Read",
			Type:                      "Allow",
			Source:                    "Explicit; effective via Finance",
			AccountName:               "alice",
			Email:                     "alice@example.com",
			OriginatingGroup:          "Finance",
			GroupInheritanceHierarchy: "Finance > Readers",
		}},
	}
	service := NewWithDependencies(stubScanner, &stubSessionRepository{})

	response, err := service.Run(Request{
		Path:                        `C:\Finance`,
		MaxDepth:                    2,
		IncludeInherited:            true,
		Context:                     context.Background(),
		EffectivePermissionExpander: expander,
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.True(t, expander.called)
	assert.Len(t, response.Permissions, 1)
	assert.Equal(t, `alice`, response.Permissions[0].Trustee)
	assert.Equal(t, "Finance", response.Permissions[0].OriginatingGroup)
	assert.Equal(t, "alice@example.com", response.Permissions[0].Email)
	assert.Equal(t, 1, response.PermissionCount)
}

func TestRunFallsBackToRawPermissionsWhenExpanderFails(t *testing.T) {
	originalSID := "S-1-5-21-1-2-3-1001"
	repository := &stubSessionRepository{}
	service := NewWithDependencies(&stubDirectoryScanner{result: &scanner.Result{
		RootPath:     `C:\Finance`,
		ItemsScanned: 1,
		Permissions: []scanner.Permission{{
			Path:       `C:\Finance`,
			Trustee:    originalSID,
			TrusteeSID: originalSID,
			Rights:     "Read",
			Type:       "Allow",
		}},
	}}, repository)

	response, err := service.Run(Request{
		Path:                        `C:\Finance`,
		Context:                     context.Background(),
		EffectivePermissionExpander: &stubEffectivePermissionExpander{err: errors.New("ldap unavailable")},
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Len(t, response.Permissions, 1)
	assert.Equal(t, originalSID, response.Permissions[0].TrusteeSID)
	assert.Equal(t, "raw", response.Permissions[0].ResolutionSource)
	assert.Equal(t, "resolver_error", response.Permissions[0].ResolutionReason)
	assert.Equal(t, "raw-fallback", response.IdentityResolution.Mode)
	assert.Equal(t, 1, response.IdentityResolution.UnresolvedPrincipalCount)
	assert.Equal(t, 1, repository.completeCalls)
	assert.Equal(t, 0, repository.failedCalls)
}

func TestRunFallsBackToRawPermissionsWhenExpanderReturnsEmptyResult(t *testing.T) {
	originalSID := "S-1-5-21-1-2-3-1001"
	repository := &stubSessionRepository{}
	service := NewWithDependencies(&stubDirectoryScanner{result: &scanner.Result{
		RootPath:     `C:\Finance`,
		ItemsScanned: 1,
		Permissions:  []scanner.Permission{{Path: `C:\Finance`, Trustee: originalSID, TrusteeSID: originalSID}},
	}}, repository)

	response, err := service.Run(Request{
		Path:                        `C:\Finance`,
		EffectivePermissionExpander: &stubEffectivePermissionExpander{expanded: []scanner.Permission{}},
	})

	require.NoError(t, err)
	require.Len(t, response.Permissions, 1)
	assert.Equal(t, originalSID, response.Permissions[0].TrusteeSID)
	assert.Equal(t, "empty_result", response.Permissions[0].ResolutionReason)
	assert.Equal(t, "raw-fallback", response.IdentityResolution.Mode)
	assert.Equal(t, 1, repository.completeCalls)
	assert.Equal(t, 0, repository.failedCalls)
}

func TestRunFallsBackToRawPermissionsWhenExpanderFactoryFails(t *testing.T) {
	repository := &stubSessionRepository{}
	service := NewWithDependencies(&stubDirectoryScanner{result: &scanner.Result{
		RootPath:     `C:\Finance`,
		ItemsScanned: 1,
		Permissions:  []scanner.Permission{{Path: `C:\Finance`, Trustee: "S-1-1-0", TrusteeSID: "S-1-1-0"}},
	}}, repository)

	response, err := service.Run(Request{
		Path: `C:\Finance`,
		EffectivePermissionExpanderFactory: func() (EffectivePermissionExpander, error) {
			return nil, errors.New("cannot connect")
		},
	})

	require.NoError(t, err)
	require.Len(t, response.Permissions, 1)
	assert.Equal(t, "raw-fallback", response.IdentityResolution.Mode)
	assert.Equal(t, 1, repository.completeCalls)
	assert.Equal(t, 0, repository.failedCalls)
}

func TestRunMarksSessionFailedWhenScannerReturnsError(t *testing.T) {
	scanErr := errors.New("scanner unavailable")
	repository := &stubSessionRepository{}
	service := NewWithDependencies(&stubDirectoryScanner{err: scanErr}, repository)

	progressEvents := make([]ProgressEvent, 0, 2)
	response, err := service.Run(Request{
		ScanID: "scan-err",
		Path:   `C:\Broken`,
		Progress: func(event ProgressEvent) {
			progressEvents = append(progressEvents, event)
		},
	})

	assert.Nil(t, response)
	assert.EqualError(t, err, scanErr.Error())
	require.Len(t, progressEvents, 2)
	assert.Equal(t, "running", progressEvents[0].Status)
	assert.Equal(t, "failed", progressEvents[1].Status)
	assert.Equal(t, scanErr.Error(), progressEvents[1].Error)
	require.NotNil(t, repository.failedSession)
	assert.Equal(t, `C:\Broken`, repository.failedSession.RootPath)
	assert.Equal(t, "failed", repository.failedSession.Status)
	assert.Equal(t, scanErr, repository.failedErr)
	require.NotNil(t, repository.failedSession.FinishedAt)
}

func TestRunMarksSessionCancelledWhenScannerReturnsCanceled(t *testing.T) {
	repository := &stubSessionRepository{}
	service := NewWithDependencies(&stubDirectoryScanner{err: context.Canceled}, repository)

	progressEvents := make([]ProgressEvent, 0, 2)
	response, err := service.Run(Request{
		ScanID: "scan-cancel",
		Path:   `C:\Broken`,
		Progress: func(event ProgressEvent) {
			progressEvents = append(progressEvents, event)
		},
	})

	assert.Nil(t, response)
	assert.ErrorIs(t, err, context.Canceled)
	require.Len(t, progressEvents, 2)
	assert.Equal(t, "running", progressEvents[0].Status)
	assert.Equal(t, "cancelled", progressEvents[1].Status)
	assert.Equal(t, "scan cancelled", progressEvents[1].Error)
	require.NotNil(t, repository.cancelledSession)
	assert.Equal(t, `C:\Broken`, repository.cancelledSession.RootPath)
	assert.Equal(t, "cancelled", repository.cancelledSession.Status)
	assert.Equal(t, 1, repository.cancelCalls)
	assert.Equal(t, 0, repository.failedCalls)
	assert.Equal(t, 0, repository.completeCalls)
}

func TestRunRejectsImmediatelyWhenConcurrencyLimitIsReached(t *testing.T) {
	blockingScanner := &blockingDirectoryScanner{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	service := newService(blockingScanner, &stubSessionRepository{}, 1)

	firstDone := make(chan error, 1)
	go func() {
		_, err := service.Run(Request{Path: `C:\First`, Context: context.Background()})
		firstDone <- err
	}()

	select {
	case <-blockingScanner.started:
	case <-time.After(time.Second):
		t.Fatal("first scan did not start")
	}

	startedAt := time.Now()
	factoryCalls := 0
	response, err := service.Run(Request{
		Path:    `C:\Second`,
		Context: context.Background(),
		EffectivePermissionExpanderFactory: func() (EffectivePermissionExpander, error) {
			factoryCalls++
			return &stubEffectivePermissionExpander{}, nil
		},
	})
	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrScanConcurrencyLimitReached)
	assert.Less(t, time.Since(startedAt), 100*time.Millisecond)
	assert.Zero(t, factoryCalls)

	close(blockingScanner.release)
	require.NoError(t, <-firstDone)
}

func TestMaxConcurrentScansFromEnvUsesPrimaryAndCompatibilityVariables(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("PERMISSION_PROTECTOR_MAX_CONCURRENT_SCANS", "")
		t.Setenv("FSA_MAX_CONCURRENT_SCANS", "")
		assert.Equal(t, 1, maxConcurrentScansFromEnv())
	})

	t.Run("primary", func(t *testing.T) {
		t.Setenv("PERMISSION_PROTECTOR_MAX_CONCURRENT_SCANS", "2")
		t.Setenv("FSA_MAX_CONCURRENT_SCANS", "3")
		assert.Equal(t, 2, maxConcurrentScansFromEnv())
	})

	t.Run("compatibility fallback", func(t *testing.T) {
		t.Setenv("PERMISSION_PROTECTOR_MAX_CONCURRENT_SCANS", "")
		t.Setenv("FSA_MAX_CONCURRENT_SCANS", "2")
		assert.Equal(t, 2, maxConcurrentScansFromEnv())
	})

	t.Run("invalid falls back to default", func(t *testing.T) {
		t.Setenv("PERMISSION_PROTECTOR_MAX_CONCURRENT_SCANS", "unlimited")
		t.Setenv("FSA_MAX_CONCURRENT_SCANS", "")
		assert.Equal(t, 1, maxConcurrentScansFromEnv())
	})
}

func TestNewWithDependenciesFallsBackToDefaultImplementations(t *testing.T) {
	service := NewWithDependencies(nil, nil)
	require.NotNil(t, service)
	assert.NotNil(t, service.scanner)
	assert.NotNil(t, service.repository)
}

func TestDatabaseSessionRepositoryNoOpsWhenDatabaseUnavailable(t *testing.T) {
	repository := &databaseSessionRepository{}
	session := &models.ScanSession{}

	assert.NoError(t, repository.CreateSession(session))
	assert.NoError(t, repository.CompleteSession(session, &Response{}))
	assert.NoError(t, repository.FailSession(session, errors.New("boom")))
}

type stubDirectoryScanner struct {
	result         *scanner.Result
	err            error
	progressEvents []scanner.Progress

	receivedPath    string
	receivedOptions scanner.Options
}

type blockingDirectoryScanner struct {
	started chan struct{}
	release chan struct{}
}

func (scannerStub *blockingDirectoryScanner) ScanDirectory(path string, options scanner.Options) (*scanner.Result, error) {
	scannerStub.started <- struct{}{}
	select {
	case <-scannerStub.release:
	case <-options.Context.Done():
		return nil, options.Context.Err()
	}

	return &scanner.Result{
		RootPath:         path,
		MaxDepth:         options.MaxDepth,
		IncludeInherited: options.IncludeInherited,
	}, nil
}

func (scannerStub *stubDirectoryScanner) ScanDirectory(path string, options scanner.Options) (*scanner.Result, error) {
	scannerStub.receivedPath = path
	scannerStub.receivedOptions = options

	for _, progressEvent := range scannerStub.progressEvents {
		if options.Progress != nil {
			options.Progress(progressEvent)
		}
	}

	if scannerStub.err != nil {
		return nil, scannerStub.err
	}

	return scannerStub.result, nil
}

type stubSessionRepository struct {
	createCalls   int
	completeCalls int
	failedCalls   int
	cancelCalls   int

	createdSession    *models.ScanSession
	completedSession  *models.ScanSession
	completedResponse *Response
	failedSession     *models.ScanSession
	cancelledSession  *models.ScanSession
	failedErr         error
}

type stubEffectivePermissionExpander struct {
	expanded []scanner.Permission
	err      error
	called   bool
}

func (expander *stubEffectivePermissionExpander) Expand(_ context.Context, _ []scanner.Permission) ([]scanner.Permission, error) {
	expander.called = true
	if expander.err != nil {
		return nil, expander.err
	}

	return append([]scanner.Permission(nil), expander.expanded...), nil
}

func (repository *stubSessionRepository) CreateSession(session *models.ScanSession) error {
	repository.createCalls++
	clonedSession := *session
	if clonedSession.ID == uuid.Nil {
		clonedSession.ID = uuid.New()
		session.ID = clonedSession.ID
	}
	repository.createdSession = &clonedSession
	return nil
}

func (repository *stubSessionRepository) CompleteSession(session *models.ScanSession, response *Response) error {
	repository.completeCalls++
	clonedSession := *session
	clonedSession.Status = "completed"
	clonedSession.ItemsScanned = response.ItemsScanned
	clonedSession.PermissionCount = response.PermissionCount
	finishedAt := response.FinishedAt
	clonedSession.FinishedAt = &finishedAt
	repository.completedSession = &clonedSession
	if response != nil {
		clonedResponse := *response
		clonedResponse.Permissions = append([]scanner.Permission(nil), response.Permissions...)
		clonedResponse.Skipped = append([]scanner.PathError(nil), response.Skipped...)
		repository.completedResponse = &clonedResponse
	}
	return nil
}

func (repository *stubSessionRepository) FailSession(session *models.ScanSession, scanErr error) error {
	repository.failedCalls++
	clonedSession := *session
	clonedSession.Status = "failed"
	finishedAt := time.Now().UTC()
	clonedSession.FinishedAt = &finishedAt
	repository.failedSession = &clonedSession
	repository.failedErr = scanErr
	return nil
}

func (repository *stubSessionRepository) CancelSession(session *models.ScanSession) error {
	repository.cancelCalls++
	clonedSession := *session
	clonedSession.Status = "cancelled"
	finishedAt := time.Now().UTC()
	clonedSession.FinishedAt = &finishedAt
	repository.cancelledSession = &clonedSession
	return nil
}
