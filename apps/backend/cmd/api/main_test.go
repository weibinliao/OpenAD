package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weibinliao/OpenAD/internal/ad"
	"github.com/weibinliao/OpenAD/internal/comparison"
	"github.com/weibinliao/OpenAD/internal/comparisonservice"
	"github.com/weibinliao/OpenAD/internal/export"
	"github.com/weibinliao/OpenAD/internal/historyservice"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/scanner"
	"github.com/weibinliao/OpenAD/internal/scanservice"
)

func TestHealthEndpoint(t *testing.T) {
	router := newTestRouter(applicationDependencies{})

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "openad", response["service"])
	assert.Equal(t, "healthy", response["status"])
}

func TestRuntimeIdentityEndpoint(t *testing.T) {
	router := newTestRouter(applicationDependencies{})

	request := httptest.NewRequest(http.MethodGet, "/api/system/runtime-identity", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.NotEmpty(t, response["account_name"])
	assert.NotEmpty(t, response["note"])
	assert.NotEmpty(t, response["goos"])
}

func TestResolveServerAddressPrefersAPIPort(t *testing.T) {
	t.Setenv("API_HOST", "")
	t.Setenv("BIND_HOST", "")
	t.Setenv("API_PORT", "18080")
	t.Setenv("PORT", "19090")

	assert.Equal(t, "127.0.0.1:18080", resolveServerAddress())
}

func TestResolveServerAddressFallsBackToPort(t *testing.T) {
	t.Setenv("API_HOST", "")
	t.Setenv("BIND_HOST", "")
	t.Setenv("API_PORT", "")
	t.Setenv("PORT", "19090")

	assert.Equal(t, "127.0.0.1:19090", resolveServerAddress())
}

func TestResolveServerAddressFallsBackToDefaultWhenInvalid(t *testing.T) {
	t.Setenv("API_HOST", "")
	t.Setenv("BIND_HOST", "")
	t.Setenv("API_PORT", "invalid")
	t.Setenv("PORT", "70000")

	assert.Equal(t, "127.0.0.1:18080", resolveServerAddress())
}

func TestResolveServerAddressPreservesExplicitAllInterfaceHosts(t *testing.T) {
	t.Setenv("BIND_HOST", "")
	t.Setenv("API_PORT", "18080")
	t.Setenv("PORT", "")

	for _, host := range []string{"0.0.0.0", "*", "+"} {
		t.Run(host, func(t *testing.T) {
			t.Setenv("API_HOST", host)
			assert.Equal(t, "0.0.0.0:18080", resolveServerAddress())
		})
	}
}

func TestNetworkAdmissionPolicyAllowsPrivateAndDeniesSpecificCIDR(t *testing.T) {
	policy, validationErrors := buildNetworkAdmissionPolicy(true, []string{"private"}, []string{"192.0.2.0/24"})
	require.Empty(t, validationErrors)

	assert.False(t, policy.Allows(net.ParseIP("198.51.100.135")))
	assert.True(t, policy.Allows(net.ParseIP("198.51.100.10")))
	assert.True(t, policy.Allows(net.ParseIP("127.0.0.1")))
}

func TestNetworkAdmissionPolicyRejectsCurrentClientLockout(t *testing.T) {
	t.Setenv("NETWORK_ACL_CONFIG", filepath.Join(t.TempDir(), "network-admission.json"))

	controller := newNetworkAdmissionController(networkAdmissionPolicy{})
	router := gin.New()
	router.PUT("/api/system/network-admission", handleUpdateNetworkAdmission(controller))

	payload := `{"enabled":true,"allow_rules":["192.0.2.0/24"],"deny_rules":[]}`
	request := httptest.NewRequest(http.MethodPut, "/api/system/network-admission", strings.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "198.51.100.135:54123"
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "would block the current client")
}

func TestCorsMiddlewareHandlesPreflightRequests(t *testing.T) {
	t.Setenv("ALLOW_ORIGINS", "")
	router := newTestRouter(applicationDependencies{})

	request := httptest.NewRequest(http.MethodOptions, "/api/scan", nil)
	request.Header.Set("Origin", "http://127.0.0.1:43110")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "http://127.0.0.1:43110", recorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", recorder.Header().Get("Vary"))
}

func TestCorsDefaultDoesNotAllowExternalOrigins(t *testing.T) {
	t.Setenv("ALLOW_ORIGINS", "")
	assert.NotContains(t, parseAllowedOrigins(""), "*")

	router := newTestRouter(applicationDependencies{})
	request := httptest.NewRequest(http.MethodOptions, "/api/scan", nil)
	request.Header.Set("Origin", "https://operator.example")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
}

func TestCorsAllowsExplicitWildcardConfiguration(t *testing.T) {
	t.Setenv("ALLOW_ORIGINS", "*")
	router := newTestRouter(applicationDependencies{})

	request := httptest.NewRequest(http.MethodOptions, "/api/scan", nil)
	request.Header.Set("Origin", "https://operator.example")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
}

func TestSecurityHeadersMiddlewareAddsHeaders(t *testing.T) {
	router := newTestRouter(applicationDependencies{})

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", recorder.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", recorder.Header().Get("Referrer-Policy"))
	assert.Equal(t, "no-store, no-cache, must-revalidate", recorder.Header().Get("Cache-Control"))
}

func TestEndpointProtectionMiddlewareRateLimitsProtectedRoutes(t *testing.T) {
	originalLimiter := endpointLimiter
	endpointLimiter = newWindowRateLimiter(1, time.Hour)
	defer func() {
		endpointLimiter = originalLimiter
	}()

	router := newTestRouter(applicationDependencies{})

	first := performJSONRequestToRouter(t, router, "/api/scan", ScanRequest{})
	assert.Equal(t, http.StatusBadRequest, first.Code)

	second := performJSONRequestToRouter(t, router, "/api/scan", ScanRequest{})
	assert.Equal(t, http.StatusTooManyRequests, second.Code)
	assert.Contains(t, second.Body.String(), "rate limit exceeded")
}

func TestScanEndpointStreamsProgressAndReturnsResponse(t *testing.T) {
	startedAt := time.Now().Add(-1 * time.Minute).UTC()
	finishedAt := time.Now().UTC()
	scanRunner := &stubScanRunner{
		runFunc: func(request scanservice.Request) (*scanservice.Response, error) {
			require.Equal(t, "scan-123", request.ScanID)
			require.Equal(t, `C:\Finance`, request.Path)
			require.Equal(t, 2, request.MaxDepth)
			require.True(t, request.IncludeInherited)
			require.NotNil(t, request.Progress)

			request.Progress(scanservice.ProgressEvent{
				ScanID:          request.ScanID,
				SessionID:       "session-123",
				ItemsScanned:    2,
				PermissionCount: 1,
				CurrentPath:     `C:\Finance`,
				Status:          "running",
			})
			request.Progress(scanservice.ProgressEvent{
				ScanID:          request.ScanID,
				SessionID:       "session-123",
				ItemsScanned:    4,
				PermissionCount: 1,
				CurrentPath:     `C:\Finance`,
				Status:          "completed",
			})

			return &scanservice.Response{
				SessionID:        "session-123",
				RootPath:         `C:\Finance`,
				MaxDepth:         2,
				IncludeInherited: true,
				ItemsScanned:     4,
				PermissionCount:  1,
				StartedAt:        startedAt,
				FinishedAt:       finishedAt,
				Permissions: []scanner.Permission{{
					Path:       `C:\Finance`,
					Trustee:    `DOMAIN\Alice`,
					TrusteeSID: "S-1-5-21-100",
					Rights:     "Read",
					Type:       "Allow",
				}},
			}, nil
		},
	}

	server := httptest.NewServer(newTestRouter(applicationDependencies{scans: scanRunner}))
	defer server.Close()

	connection, receivedEvents := connectScanProgressSocket(t, server.URL, "scan-123")
	defer connection.Close()

	payload := ScanRequest{
		Path:             `C:\Finance`,
		Depth:            2,
		IncludeInherited: boolPtr(true),
		ScanID:           "scan-123",
	}
	recorder := performJSONRequest(t, server.URL+"/api/scan", payload)
	assert.Equal(t, http.StatusOK, recorder.StatusCode)

	var response scanservice.Response
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))
	assert.Equal(t, "session-123", response.SessionID)
	assert.Equal(t, 4, response.ItemsScanned)
	assert.Len(t, response.Permissions, 1)

	events := collectProgressEvents(t, receivedEvents)
	require.Len(t, events, 3)
	assert.Equal(t, "connected", events[0].Status)
	assert.Equal(t, "running", events[1].Status)
	assert.Equal(t, "completed", events[2].Status)
	assert.Equal(t, "session-123", events[2].SessionID)
	assert.Equal(t, "scan-123", events[2].ScanID)
}

func TestScanProgressWebSocketOriginAdmission(t *testing.T) {
	server := httptest.NewServer(newTestRouter(applicationDependencies{}))
	defer server.Close()

	parsedURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	parsedURL.Scheme = "ws"
	parsedURL.Path = "/api/scan/ws"
	parsedURL.RawQuery = "scan_id=origin-check"

	t.Run("desktop loopback origin connects", func(t *testing.T) {
		headers := http.Header{"Origin": []string{"http://127.0.0.1:43110"}}
		connection, response, err := websocket.DefaultDialer.Dial(parsedURL.String(), headers)
		require.NoError(t, err)
		require.NotNil(t, connection)
		assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
		require.NoError(t, connection.Close())
	})

	t.Run("external origin is rejected", func(t *testing.T) {
		headers := http.Header{"Origin": []string{"https://evil.example"}}
		connection, response, err := websocket.DefaultDialer.Dial(parsedURL.String(), headers)
		if connection != nil {
			_ = connection.Close()
		}
		require.Error(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusForbidden, response.StatusCode)
	})
}

func TestScanEndpointForwardsEffectivePermissionExpansionRequest(t *testing.T) {
	startedAt := time.Now().Add(-1 * time.Minute).UTC()
	finishedAt := time.Now().UTC()
	expander := &stubEffectivePermissionExpander{
		expandedPermissions: []scanner.Permission{{
			Path:       `C:\Finance`,
			Trustee:    `alice`,
			TrusteeSID: "S-1-5-21-user",
			Rights:     "Read",
			Type:       "Allow",
			Source:     "Explicit; effective via Finance",
		}},
	}
	directoryService := &stubADService{permissionExpander: expander}
	scanRunner := &stubScanRunner{
		runFunc: func(request scanservice.Request) (*scanservice.Response, error) {
			require.NotNil(t, request.EffectivePermissionExpanderFactory)
			require.NotNil(t, request.Context)
			expander, err := request.EffectivePermissionExpanderFactory()
			require.NoError(t, err)
			expanded, err := expander.Expand(request.Context, []scanner.Permission{{
				Path:       `C:\Finance`,
				Trustee:    `DOMAIN\Finance`,
				TrusteeSID: "S-1-5-21-group",
				Rights:     "Read",
				Type:       "Allow",
			}})
			require.NoError(t, err)

			return &scanservice.Response{
				SessionID:        "session-123",
				RootPath:         `C:\Finance`,
				MaxDepth:         2,
				IncludeInherited: true,
				ItemsScanned:     4,
				PermissionCount:  len(expanded),
				StartedAt:        startedAt,
				FinishedAt:       finishedAt,
				Permissions:      expanded,
			}, nil
		},
	}

	server := httptest.NewServer(newTestRouter(applicationDependencies{scans: scanRunner, ad: directoryService}))
	defer server.Close()

	payload := ScanRequest{
		Path:             `C:\Finance`,
		Depth:            2,
		IncludeInherited: boolPtr(true),
		ScanID:           "scan-123",
		EffectivePermissions: &EffectivePermissionRequest{
			Enabled:              true,
			Server:               "ldap://directory.example.com",
			BaseDN:               "DC=example,DC=com",
			Username:             "svc-reader",
			Password:             "secret",
			ExcludeGroupPatterns: []string{"BUILTIN\\Users"},
			ExcludeUserPatterns:  []string{"*svc*"},
		},
	}

	recorder := performJSONRequest(t, server.URL+"/api/scan", payload)
	assert.Equal(t, http.StatusOK, recorder.StatusCode)
	assert.True(t, expander.called)
	assert.Len(t, expander.lastPermissions, 1)
}

func TestScanEndpointUsesEffectivePermissionExpansionWhenCredentialsArePresent(t *testing.T) {
	startedAt := time.Now().Add(-1 * time.Minute).UTC()
	finishedAt := time.Now().UTC()
	expander := &stubEffectivePermissionExpander{
		expandedPermissions: []scanner.Permission{{
			Path:       `C:\Finance`,
			Trustee:    `alice`,
			TrusteeSID: "S-1-5-21-user",
			Rights:     "Read",
			Type:       "Allow",
			Source:     "Explicit; effective via Finance",
		}},
	}
	directoryService := &stubADService{permissionExpander: expander}
	scanRunner := &stubScanRunner{
		runFunc: func(request scanservice.Request) (*scanservice.Response, error) {
			require.NotNil(t, request.EffectivePermissionExpanderFactory)
			expander, err := request.EffectivePermissionExpanderFactory()
			require.NoError(t, err)
			expanded, err := expander.Expand(request.Context, []scanner.Permission{{
				Path:       `C:\Finance`,
				Trustee:    `DOMAIN\Finance`,
				TrusteeSID: "S-1-5-21-group",
				Rights:     "Read",
				Type:       "Allow",
			}})
			require.NoError(t, err)

			return &scanservice.Response{
				SessionID:        "session-123",
				RootPath:         `C:\Finance`,
				MaxDepth:         2,
				IncludeInherited: true,
				ItemsScanned:     4,
				PermissionCount:  len(expanded),
				StartedAt:        startedAt,
				FinishedAt:       finishedAt,
				Permissions:      expanded,
			}, nil
		},
	}

	server := httptest.NewServer(newTestRouter(applicationDependencies{scans: scanRunner, ad: directoryService}))
	defer server.Close()

	payload := ScanRequest{
		Path:             `C:\Finance`,
		Depth:            2,
		IncludeInherited: boolPtr(true),
		ScanID:           "scan-123",
		EffectivePermissions: &EffectivePermissionRequest{
			Enabled:  false,
			Server:   "ldap://directory.example.com",
			BaseDN:   "DC=example,DC=com",
			Username: "svc-reader",
			Password: "secret",
		},
	}

	recorder := performJSONRequest(t, server.URL+"/api/scan", payload)
	assert.Equal(t, http.StatusOK, recorder.StatusCode)
	assert.True(t, expander.called)
}

func TestScanEndpointValidatesRequestBody(t *testing.T) {
	router := newTestRouter(applicationDependencies{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/scan", bytes.NewBufferString(`{}`))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestScanEndpointRejectsIncompleteEffectivePermissionRequest(t *testing.T) {
	router := newTestRouter(applicationDependencies{})
	recorder := performJSONRequestToRouter(t, router, "/api/scan", ScanRequest{
		Path: `C:\Finance`,
		EffectivePermissions: &EffectivePermissionRequest{
			Enabled:  true,
			Server:   "ldap://directory.example.com",
			BaseDN:   "DC=example,DC=com",
			Username: "svc-reader",
			// Missing password
		},
	})

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "effective_permissions.password")
}

func TestExportEndpointHandlesCSVAndUnsupportedFormats(t *testing.T) {
	exportDir := t.TempDir()
	t.Setenv("PERMISSION_PROTECTOR_EXPORT_DIR", exportDir)
	exporter := &stubExporter{}
	router := newTestRouter(applicationDependencies{exporter: exporter})

	t.Run("csv success", func(t *testing.T) {
		recorder := performJSONRequestToRouter(t, router, "/api/export", ExportRequest{
			Permissions: []models.Permission{{Path: `C:\Finance`, Trustee: `DOMAIN\Alice`}},
			Format:      "csv",
			Filename:    "finance.csv",
		})

		assert.Equal(t, http.StatusOK, recorder.Code)
		require.NotNil(t, exporter.lastCSV)
		assert.Equal(t, filepath.Join(exportDir, "finance.csv"), exporter.lastCSV.filename)
		assert.Len(t, exporter.lastCSV.permissions, 1)
	})

	t.Run("html success", func(t *testing.T) {
		recorder := performJSONRequestToRouter(t, router, "/api/export", ExportRequest{
			Permissions: []models.Permission{{Path: `C:\Finance`, Trustee: `DOMAIN\Alice`}},
			Format:      "html",
			Filename:    "finance.html",
		})

		assert.Equal(t, http.StatusOK, recorder.Code)
		require.NotNil(t, exporter.lastHTML)
		assert.Equal(t, filepath.Join(exportDir, "finance.html"), exporter.lastHTML.filename)
		assert.Len(t, exporter.lastHTML.permissions, 1)
	})

	t.Run("unsupported format", func(t *testing.T) {
		recorder := performJSONRequestToRouter(t, router, "/api/export", ExportRequest{
			Permissions: []models.Permission{{Path: `C:\Finance`}},
			Format:      "pdf",
			Filename:    "finance.pdf",
		})

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}

func TestADTestEndpointHandlesSuccessAndFailure(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		directoryService := &stubADService{
			connectionClient: &stubADConnectionClient{},
		}
		router := newTestRouter(applicationDependencies{ad: directoryService})

		recorder := performJSONRequestToRouter(t, router, "/api/ad/test", ADConnectionRequest{
			ADCredentials: ADCredentials{
				Server:   "ldap://directory.example.com",
				BaseDN:   "DC=example,DC=com",
				Username: "svc-reader",
				Password: "secret",
			},
		})

		assert.Equal(t, http.StatusOK, recorder.Code)
		require.NotNil(t, directoryService.connectionClient)
		assert.True(t, directoryService.connectionClient.connected)
		assert.True(t, directoryService.connectionClient.closed)
	})

	t.Run("connect failure", func(t *testing.T) {
		directoryService := &stubADService{
			connectionClient: &stubADConnectionClient{connectErr: errors.New("invalid credentials")},
		}
		router := newTestRouter(applicationDependencies{ad: directoryService})

		recorder := performJSONRequestToRouter(t, router, "/api/ad/test", ADConnectionRequest{
			ADCredentials: ADCredentials{
				Server:   "ldap://directory.example.com",
				BaseDN:   "DC=example,DC=com",
				Username: "svc-reader",
				Password: "wrong-secret",
			},
		})

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}

func TestADUserQueryEndpointHandlesSuccessAndQueryErrors(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		directoryService := &stubADService{
			userSearchClient: &stubADUserSearchClient{
				users: []ad.User{{
					DN:          "CN=Alice,OU=Users,DC=example,DC=com",
					Username:    "alice",
					DisplayName: "Alice Example",
					Email:       "alice@example.com",
				}},
			},
		}
		router := newTestRouter(applicationDependencies{ad: directoryService})

		recorder := performJSONRequestToRouter(t, router, "/api/ad/users/query", ADUserQueryRequest{
			ADCredentials: ADCredentials{
				Server:   "ldap://directory.example.com",
				BaseDN:   "DC=example,DC=com",
				Username: "svc-reader",
				Password: "secret",
			},
			Query: "alice",
			Limit: 10,
		})

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "alice", directoryService.userSearchClient.lastQuery)
		assert.Equal(t, 10, directoryService.userSearchClient.lastLimit)
		assert.True(t, directoryService.userSearchClient.closed)
	})

	t.Run("query failure", func(t *testing.T) {
		directoryService := &stubADService{
			userSearchClient: &stubADUserSearchClient{searchErr: errors.New("directory unavailable")},
		}
		router := newTestRouter(applicationDependencies{ad: directoryService})

		recorder := performJSONRequestToRouter(t, router, "/api/ad/users/query", ADUserQueryRequest{
			ADCredentials: ADCredentials{
				Server:   "ldap://directory.example.com",
				BaseDN:   "DC=example,DC=com",
				Username: "svc-reader",
				Password: "secret",
			},
			Query: "alice",
		})

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})

	t.Run("client creation failure", func(t *testing.T) {
		directoryService := &stubADService{userClientErr: errors.New("cannot connect")}
		router := newTestRouter(applicationDependencies{ad: directoryService})

		recorder := performJSONRequestToRouter(t, router, "/api/ad/users/query", ADUserQueryRequest{
			ADCredentials: ADCredentials{
				Server:   "ldap://directory.example.com",
				BaseDN:   "DC=example,DC=com",
				Username: "svc-reader",
				Password: "secret",
			},
			Query: "alice",
		})

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}

func TestADUserAsyncQueryLifecycle(t *testing.T) {
	directoryService := &stubADService{
		userSearchClient: &stubADUserSearchClient{
			users: []ad.User{
				{DN: "CN=Alice,OU=Users,DC=example,DC=com", Username: "alice", DisplayName: "Alice"},
				{DN: "CN=Bob,OU=Users,DC=example,DC=com", Username: "bob", DisplayName: "Bob"},
			},
		},
	}
	router := newTestRouter(applicationDependencies{ad: directoryService})

	createRecorder := performJSONRequestToRouter(t, router, "/api/ad/users/query/async", ADUserAsyncQueryRequest{
		ADCredentials: ADCredentials{
			Server:   "ldap://directory.example.com",
			BaseDN:   "DC=example,DC=com",
			Username: "svc-reader",
			Password: "secret",
		},
		Query: "a",
		Limit: 25,
	})
	require.Equal(t, http.StatusAccepted, createRecorder.Code)

	var createResponse struct {
		JobID string `json:"job_id"`
	}
	require.NoError(t, json.Unmarshal(createRecorder.Body.Bytes(), &createResponse))
	require.NotEmpty(t, createResponse.JobID)

	var finalBody string
	for attempt := 0; attempt < 30; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/ad/jobs/"+createResponse.JobID, nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code)
		finalBody = recorder.Body.String()
		if strings.Contains(finalBody, `"status":"completed"`) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	assert.Contains(t, finalBody, `"status":"completed"`)
	assert.Contains(t, finalBody, `"total_users":2`)
}

func TestADJobListAndCancelEndpoints(t *testing.T) {
	now := time.Now().UTC()
	adQueryJobs.Set(adQueryJob{
		ID:        "test-job-queued",
		Status:    "queued",
		Progress:  0,
		Query:     "alice",
		CreatedAt: now,
		UpdatedAt: now,
		Request: ADUserAsyncQueryRequest{
			ADCredentials: ADCredentials{Server: "ldap://directory.example.com", BaseDN: "DC=example,DC=com", Username: "svc", Password: "secret"}, Query: "alice", Limit: 20,
		},
	})
	adQueryJobs.Set(adQueryJob{
		ID:        "test-job-failed",
		Status:    "failed",
		Progress:  100,
		Query:     "bob",
		CreatedAt: now,
		UpdatedAt: now,
		Error:     "query failed",
		Request: ADUserAsyncQueryRequest{
			ADCredentials: ADCredentials{Server: "ldap://directory.example.com", BaseDN: "DC=example,DC=com", Username: "svc", Password: "secret"}, Query: "bob", Limit: 20,
		},
	})

	router := newTestRouter(applicationDependencies{})

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/ad/jobs?page=1&page_size=20&status=failed", nil)
	router.ServeHTTP(listRecorder, listRequest)
	assert.Equal(t, http.StatusOK, listRecorder.Code)
	assert.Contains(t, listRecorder.Body.String(), `"id":"test-job-failed"`)

	cancelRecorder := httptest.NewRecorder()
	cancelRequest := httptest.NewRequest(http.MethodPost, "/api/ad/jobs/test-job-queued/cancel", nil)
	router.ServeHTTP(cancelRecorder, cancelRequest)
	assert.Equal(t, http.StatusOK, cancelRecorder.Code)
	assert.Contains(t, cancelRecorder.Body.String(), `"status":"cancelled"`)
}

func TestADJobRetryEndpoint(t *testing.T) {
	now := time.Now().UTC()
	adQueryJobs.Set(adQueryJob{
		ID:        "test-job-retry-source",
		Status:    "failed",
		Progress:  100,
		Query:     "alice",
		CreatedAt: now,
		UpdatedAt: now,
		Error:     "query failed",
		Request: ADUserAsyncQueryRequest{
			ADCredentials: ADCredentials{Server: "ldap://directory.example.com", BaseDN: "DC=example,DC=com", Username: "svc", Password: "secret"}, Query: "alice", Limit: 20,
		},
	})
	directoryService := &stubADService{
		userSearchClient: &stubADUserSearchClient{
			users: []ad.User{{DN: "CN=Alice,DC=example,DC=com", Username: "alice", DisplayName: "Alice"}},
		},
	}
	router := newTestRouter(applicationDependencies{ad: directoryService})

	retryRecorder := httptest.NewRecorder()
	retryRequest := httptest.NewRequest(http.MethodPost, "/api/ad/jobs/test-job-retry-source/retry", nil)
	router.ServeHTTP(retryRecorder, retryRequest)
	assert.Equal(t, http.StatusAccepted, retryRecorder.Code)

	var retryResponse struct {
		JobID string `json:"job_id"`
	}
	require.NoError(t, json.Unmarshal(retryRecorder.Body.Bytes(), &retryResponse))
	require.NotEmpty(t, retryResponse.JobID)

	var finalBody string
	for attempt := 0; attempt < 30; attempt++ {
		jobRecorder := httptest.NewRecorder()
		jobRequest := httptest.NewRequest(http.MethodGet, "/api/ad/jobs/"+retryResponse.JobID, nil)
		router.ServeHTTP(jobRecorder, jobRequest)
		require.Equal(t, http.StatusOK, jobRecorder.Code)
		finalBody = jobRecorder.Body.String()
		if strings.Contains(finalBody, `"status":"completed"`) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	assert.Contains(t, finalBody, `"status":"completed"`)
}

func TestListSessionsEndpointForwardsFilters(t *testing.T) {
	history := &stubHistoryService{
		listSessionsResponse: &historyservice.SessionListResponse{
			Items:      []models.ScanSession{{ID: uuidMust(t, "9fd31d1d-7a42-46e4-a6bd-3de3d4f9164a"), RootPath: `C:\Finance`, Status: "completed"}},
			Pagination: historyservice.Pagination{Page: 2, PageSize: 5, Total: 1, TotalPages: 1},
		},
	}
	router := newTestRouter(applicationDependencies{history: history})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/sessions?page=2&page_size=5&status=completed&root_path=finance", nil)
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, 2, history.lastListSessionsFilter.Page)
	assert.Equal(t, 5, history.lastListSessionsFilter.PageSize)
	assert.Equal(t, "completed", history.lastListSessionsFilter.Status)
	assert.Equal(t, "finance", history.lastListSessionsFilter.RootPath)
}

func TestListSessionsEndpointHandlesValidationAndServiceErrors(t *testing.T) {
	t.Run("invalid page", func(t *testing.T) {
		router := newTestRouter(applicationDependencies{history: &stubHistoryService{}})

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/sessions?page=0", nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("service unavailable", func(t *testing.T) {
		history := &stubHistoryService{listSessionsErr: historyservice.ErrDatabaseUnavailable}
		router := newTestRouter(applicationDependencies{history: history})

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	})
}

func TestGetSessionEndpointHandlesSuccessAndNotFound(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sessionID := uuid.NewString()
		history := &stubHistoryService{
			getSessionResponse: &models.ScanSession{ID: uuidMust(t, sessionID), RootPath: `C:\Finance`, Status: "completed"},
		}
		router := newTestRouter(applicationDependencies{history: history})

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/sessions/"+sessionID, nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, sessionID, history.lastGetSessionID)
	})

	t.Run("not found", func(t *testing.T) {
		history := &stubHistoryService{getSessionErr: historyservice.ErrSessionNotFound}
		router := newTestRouter(applicationDependencies{history: history})

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/sessions/"+uuid.NewString(), nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("invalid session id", func(t *testing.T) {
		router := newTestRouter(applicationDependencies{history: &stubHistoryService{getSessionErr: historyservice.ErrInvalidSessionID}})

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/sessions/not-a-uuid", nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}

func TestListSessionPermissionsEndpointForwardsFiltersAndValidatesQuery(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sessionID := uuid.NewString()
		inherited := true
		history := &stubHistoryService{
			listPermissionsResponse: &historyservice.PermissionListResponse{
				Items:      []models.Permission{{ID: uuid.New(), Trustee: `DOMAIN\Bob`, Path: `C:\Finance\Payroll`, Inherited: true}},
				Pagination: historyservice.Pagination{Page: 1, PageSize: 10, Total: 1, TotalPages: 1},
			},
		}
		router := newTestRouter(applicationDependencies{history: history})

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/sessions/"+sessionID+"/permissions?page=1&page_size=10&path=payroll&trustee=bob&inherited=true", nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, sessionID, history.lastListPermissionsID)
		assert.Equal(t, "payroll", history.lastListPermissionsFilter.Path)
		assert.Equal(t, "bob", history.lastListPermissionsFilter.Trustee)
		if assert.NotNil(t, history.lastListPermissionsFilter.Inherited) {
			assert.Equal(t, inherited, *history.lastListPermissionsFilter.Inherited)
		}
	})

	t.Run("invalid inherited filter", func(t *testing.T) {
		router := newTestRouter(applicationDependencies{history: &stubHistoryService{}})

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/sessions/"+uuid.NewString()+"/permissions?inherited=maybe", nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("service failure", func(t *testing.T) {
		history := &stubHistoryService{listPermissionsErr: errors.New("unexpected failure")}
		router := newTestRouter(applicationDependencies{history: history})

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/sessions/"+uuid.NewString()+"/permissions", nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	})
}

func TestCompareEndpointInvokesComparisonService(t *testing.T) {
	report := &comparison.ChangeReport{
		BaselineID:   uuid.NewString(),
		CurrentID:    uuid.NewString(),
		ChangesCount: 1,
	}
	comparisonService := &stubComparisonService{report: report}
	router := newTestRouter(applicationDependencies{comparison: comparisonService})

	recorder := performJSONRequestToRouter(t, router, "/api/compare", CompareRequest{
		BaselineSessionID: report.BaselineID,
		CurrentSessionID:  report.CurrentID,
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, comparisonService.lastRequest)
	assert.Equal(t, report.BaselineID, comparisonService.lastRequest.BaselineSessionID)
	assert.Equal(t, report.CurrentID, comparisonService.lastRequest.CurrentSessionID)
}

func TestCompareEndpointHandlesValidationAndLookupErrors(t *testing.T) {
	t.Run("invalid session id", func(t *testing.T) {
		router := newTestRouter(applicationDependencies{comparison: &stubComparisonService{err: historyservice.ErrInvalidSessionID}})

		recorder := performJSONRequestToRouter(t, router, "/api/compare", CompareRequest{
			BaselineSessionID: "not-a-uuid",
			CurrentSessionID:  uuid.NewString(),
		})

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("not found", func(t *testing.T) {
		router := newTestRouter(applicationDependencies{comparison: &stubComparisonService{err: historyservice.ErrSessionNotFound}})

		recorder := performJSONRequestToRouter(t, router, "/api/compare", CompareRequest{
			BaselineSessionID: uuid.NewString(),
			CurrentSessionID:  uuid.NewString(),
		})

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})
}

func TestListSessionChangesEndpointForwardsFiltersAndValidatesQuery(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sessionID := uuid.NewString()
		history := &stubHistoryService{
			listChangesResponse: &historyservice.ChangeListResponse{
				Items: []models.PermissionChange{{
					ID:         uuid.New(),
					Path:       `C:\Finance\Payroll`,
					ChangeType: "added",
					DetectedAt: time.Now().UTC(),
				}},
				Pagination: historyservice.Pagination{Page: 1, PageSize: 10, Total: 1, TotalPages: 1},
			},
		}
		router := newTestRouter(applicationDependencies{history: history})

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/sessions/"+sessionID+"/changes?page=1&page_size=10&change_type=added&path=payroll", nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, sessionID, history.lastListChangesID)
		assert.Equal(t, "added", history.lastListChangesFilter.ChangeType)
		assert.Equal(t, "payroll", history.lastListChangesFilter.Path)
	})

	t.Run("service failure", func(t *testing.T) {
		history := &stubHistoryService{listChangesErr: errors.New("unexpected failure")}
		router := newTestRouter(applicationDependencies{history: history})

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/sessions/"+uuid.NewString()+"/changes", nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	})
}

func newTestRouter(dependencies applicationDependencies) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return newApplication(dependencies).router()
}

func connectScanProgressSocket(t *testing.T, serverURL, scanID string) (*websocket.Conn, <-chan scanservice.ProgressEvent) {
	t.Helper()

	parsedURL, err := url.Parse(serverURL)
	require.NoError(t, err)
	parsedURL.Scheme = "ws"
	parsedURL.Path = "/api/scan/ws"
	parsedURL.RawQuery = "scan_id=" + url.QueryEscape(scanID)

	connection, _, err := websocket.DefaultDialer.Dial(parsedURL.String(), nil)
	require.NoError(t, err)

	events := make(chan scanservice.ProgressEvent, 8)
	go func() {
		defer close(events)
		for {
			var event scanservice.ProgressEvent
			if err := connection.ReadJSON(&event); err != nil {
				return
			}

			events <- event
		}
	}()

	return connection, events
}

func collectProgressEvents(t *testing.T, events <-chan scanservice.ProgressEvent) []scanservice.ProgressEvent {
	t.Helper()

	collected := make([]scanservice.ProgressEvent, 0, 4)
	timeout := time.After(5 * time.Second)

	for len(collected) < 3 {
		select {
		case event, ok := <-events:
			if !ok {
				return collected
			}
			collected = append(collected, event)
		case <-timeout:
			require.FailNow(t, "timed out waiting for progress events")
		}
	}

	return collected
}

func performJSONRequest(t *testing.T, requestURL string, payload any) *http.Response {
	t.Helper()

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	response, err := http.Post(requestURL, "application/json", bytes.NewBuffer(data))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = response.Body.Close()
	})

	return response
}

func performJSONRequestToRouter(t *testing.T, router *gin.Engine, target string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, target, bytes.NewBuffer(data))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder
}

func performJSONRequestWithMethodToRouter(t *testing.T, router *gin.Engine, method string, target string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	return performJSONRequestWithMethodAndRemoteToRouter(t, router, method, target, payload, "192.0.2.1:1234")
}

func performJSONRequestWithMethodAndRemoteToRouter(t *testing.T, router *gin.Engine, method string, target string, payload any, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	request := httptest.NewRequest(method, target, bytes.NewBuffer(data))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = remoteAddr
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder
}

func performRequestToRouter(t *testing.T, router *gin.Engine, method string, target string, body *bytes.Buffer, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	if body == nil {
		body = bytes.NewBuffer(nil)
	}

	request := httptest.NewRequest(method, target, body)
	request.RemoteAddr = remoteAddr
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder
}

func boolPtr(value bool) *bool {
	return &value
}

func TestDefaultADServiceBuildsConnectionClient(t *testing.T) {
	client := defaultADService{}.NewConnectionClient("ldap://directory.example.com", "DC=example,DC=com", "svc-reader", "secret")
	assert.NotNil(t, client)
}

func uuidMust(t *testing.T, value string) uuid.UUID {
	t.Helper()

	parsed, err := uuid.Parse(value)
	require.NoError(t, err)

	return parsed
}

type stubScanRunner struct {
	runFunc func(request scanservice.Request) (*scanservice.Response, error)
}

func (runner *stubScanRunner) Run(request scanservice.Request) (*scanservice.Response, error) {
	if runner.runFunc == nil {
		return nil, nil
	}

	return runner.runFunc(request)
}

type stubHistoryService struct {
	listSessionsResponse   *historyservice.SessionListResponse
	listSessionsErr        error
	lastListSessionsFilter historyservice.SessionListFilter

	getSessionResponse *models.ScanSession
	getSessionErr      error
	lastGetSessionID   string

	getSessionBundleResponse *historyservice.SessionBundleResponse
	getSessionBundleErr      error
	lastGetSessionBundleID   string

	listPermissionsResponse   *historyservice.PermissionListResponse
	listPermissionsErr        error
	lastListPermissionsID     string
	lastListPermissionsFilter historyservice.PermissionListFilter

	listChangesResponse   *historyservice.ChangeListResponse
	listChangesErr        error
	lastListChangesID     string
	lastListChangesFilter historyservice.ChangeListFilter
}

func (service *stubHistoryService) ListSessions(filter historyservice.SessionListFilter) (*historyservice.SessionListResponse, error) {
	service.lastListSessionsFilter = filter
	return service.listSessionsResponse, service.listSessionsErr
}

func (service *stubHistoryService) GetSession(id string) (*models.ScanSession, error) {
	service.lastGetSessionID = id
	return service.getSessionResponse, service.getSessionErr
}

func (service *stubHistoryService) GetSessionBundle(id string) (*historyservice.SessionBundleResponse, error) {
	service.lastGetSessionBundleID = id
	return service.getSessionBundleResponse, service.getSessionBundleErr
}

func (service *stubHistoryService) ListSessionPermissions(id string, filter historyservice.PermissionListFilter) (*historyservice.PermissionListResponse, error) {
	service.lastListPermissionsID = id
	service.lastListPermissionsFilter = filter
	return service.listPermissionsResponse, service.listPermissionsErr
}

func (service *stubHistoryService) ListSessionChanges(id string, filter historyservice.ChangeListFilter) (*historyservice.ChangeListResponse, error) {
	service.lastListChangesID = id
	service.lastListChangesFilter = filter
	return service.listChangesResponse, service.listChangesErr
}

type exportCall struct {
	permissions []models.Permission
	filename    string
	options     export.Options
}

type stubExporter struct {
	lastCSV   *exportCall
	lastExcel *exportCall
	lastHTML  *exportCall
	csvErr    error
	excelErr  error
	htmlErr   error
}

func (exporter *stubExporter) ExportToCSV(permissions []models.Permission, filename string, options export.Options) error {
	exporter.lastCSV = &exportCall{permissions: append([]models.Permission(nil), permissions...), filename: filename, options: options}
	if filepath.IsAbs(filename) {
		_ = os.WriteFile(filename, []byte("Path,Trustee\nC:\\Finance,DOMAIN\\Alice\n"), 0o644)
	}
	return exporter.csvErr
}

func (exporter *stubExporter) ExportToExcel(permissions []models.Permission, filename string, options export.Options) error {
	exporter.lastExcel = &exportCall{permissions: append([]models.Permission(nil), permissions...), filename: filename, options: options}
	if filepath.IsAbs(filename) {
		_ = os.WriteFile(filename, []byte("excel-bytes"), 0o644)
	}
	return exporter.excelErr
}

func (exporter *stubExporter) ExportToHTML(permissions []models.Permission, filename string, options export.Options) error {
	exporter.lastHTML = &exportCall{permissions: append([]models.Permission(nil), permissions...), filename: filename, options: options}
	if filepath.IsAbs(filename) {
		_ = os.WriteFile(filename, []byte("<html><body>ok</body></html>"), 0o644)
	}
	return exporter.htmlErr
}

type stubADService struct {
	connectionClient        *stubADConnectionClient
	userSearchClient        *stubADUserSearchClient
	groupClient             *stubADGroupClient
	treeClient              *stubADTreeClient
	permissionExpander      scanservice.EffectivePermissionExpander
	permissionExpanderCalls int
	userClientErr           error
	groupClientErr          error
	treeClientErr           error
}

func TestExportDownloadEndpointReturnsAttachment(t *testing.T) {
	exporter := &stubExporter{}
	router := newTestRouter(applicationDependencies{exporter: exporter})

	recorder := performJSONRequestToRouter(t, router, "/api/export/download", ExportDownloadRequest{
		Permissions: []models.Permission{{Path: `C:\Finance`, Trustee: `DOMAIN\Alice`}},
		UserRows: []export.UserRow{{
			Path:             `C:\Finance`,
			Trustee:          `DOMAIN\Alice`,
			TrusteeSID:       `S-1-5-21-1000`,
			AccountName:      "alice",
			OriginatingGroup: `DOMAIN\Finance-Team`,
			Permissions:      "Allow: Read, This Folder Only",
			RowCount:         1,
			MemberKeys:       []string{"finance-read"},
		}},
		Format:      "csv",
		Mode:        "scan-results",
		Filename:    "finance-permissions",
		Title:       "权限报告 - 财务目录",
		Template:    "management",
		Sections:    []string{"metadata", "kpis"},
		ADFields:    []string{"sam", "mail"},
		FileColumns: []string{"path", "rights"},
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, recorder.Header().Get("Content-Disposition"), "finance-permissions.csv")
	assert.NotEmpty(t, recorder.Body.String())
	require.NotNil(t, exporter.lastCSV)
	assert.Equal(t, "权限报告 - 财务目录", exporter.lastCSV.options.Title)
	assert.Equal(t, "scan-results", exporter.lastCSV.options.Mode)
	assert.Equal(t, "management", exporter.lastCSV.options.Template)
	assert.Equal(t, []string{"metadata", "kpis"}, exporter.lastCSV.options.Sections)
	assert.Equal(t, []string{"sam", "mail"}, exporter.lastCSV.options.ADFields)
	assert.Equal(t, []string{"path", "rights"}, exporter.lastCSV.options.FileColumns)
	require.Len(t, exporter.lastCSV.options.UserRows, 1)
	assert.Equal(t, `C:\Finance`, exporter.lastCSV.options.UserRows[0].Path)
	assert.Equal(t, "Allow: Read, This Folder Only", exporter.lastCSV.options.UserRows[0].Permissions)
}

func TestListDirectoriesEndpointReturnsChildren(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "A"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, "B"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "file.txt"), []byte("x"), 0o644))

	router := newTestRouter(applicationDependencies{})
	target := "/api/fs/directories?path=" + url.QueryEscape(root)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `"name":"A"`)
	assert.Contains(t, body, `"name":"B"`)
	assert.False(t, strings.Contains(body, "file.txt"))
}

func TestADGroupEndpointsHandleSuccessAndFailures(t *testing.T) {
	t.Run("query groups success", func(t *testing.T) {
		directoryService := &stubADService{
			groupClient: &stubADGroupClient{
				groups: []models.ADGroup{{DN: "CN=Finance,OU=Groups,DC=example,DC=com", Name: "Finance"}},
			},
		}
		router := newTestRouter(applicationDependencies{ad: directoryService})

		recorder := performJSONRequestToRouter(t, router, "/api/ad/groups/query", ADGroupQueryRequest{
			ADCredentials: ADCredentials{
				Server:   "ldap://directory.example.com",
				BaseDN:   "DC=example,DC=com",
				Username: "svc-reader",
				Password: "secret",
			},
			Query: "finance",
			Limit: 15,
		})

		assert.Equal(t, http.StatusOK, recorder.Code)
		require.NotNil(t, directoryService.groupClient)
		assert.Equal(t, "finance", directoryService.groupClient.lastQuery)
		assert.Equal(t, 15, directoryService.groupClient.lastLimit)
		assert.True(t, directoryService.groupClient.closed)
	})

	t.Run("group members success", func(t *testing.T) {
		directoryService := &stubADService{
			groupClient: &stubADGroupClient{
				group: &models.ADGroup{
					DN:   "CN=Finance,OU=Groups,DC=example,DC=com",
					Name: "Finance",
					Members: []models.ADPrincipal{
						{DN: "CN=Alice,OU=Users,DC=example,DC=com", Name: "Alice", SAMAccountName: "alice", Type: models.ADObjectTypeUser},
					},
				},
			},
		}
		router := newTestRouter(applicationDependencies{ad: directoryService})

		recorder := performJSONRequestToRouter(t, router, "/api/ad/groups/members", ADGroupMembersRequest{
			ADCredentials: ADCredentials{
				Server:   "ldap://directory.example.com",
				BaseDN:   "DC=example,DC=com",
				Username: "svc-reader",
				Password: "secret",
			},
			GroupDN: "CN=Finance,OU=Groups,DC=example,DC=com",
		})

		assert.Equal(t, http.StatusOK, recorder.Code)
		require.NotNil(t, directoryService.groupClient)
		assert.Equal(t, "CN=Finance,OU=Groups,DC=example,DC=com", directoryService.groupClient.lastRequestedDN)
		assert.True(t, directoryService.groupClient.closed)
	})

	t.Run("group members supports nested resolution", func(t *testing.T) {
		rootDN := "CN=Finance,OU=Groups,DC=example,DC=com"
		nestedDN := "CN=Finance-Approvers,OU=Groups,DC=example,DC=com"
		aliceDN := "CN=Alice,OU=Users,DC=example,DC=com"
		bobDN := "CN=Bob,OU=Users,DC=example,DC=com"
		directoryService := &stubADService{
			groupClient: &stubADGroupClient{
				groupsByDN: map[string]models.ADGroup{
					strings.ToLower(rootDN): {
						DN:   rootDN,
						Name: "Finance",
						Members: []models.ADPrincipal{
							{DN: nestedDN, Name: "Finance-Approvers", SAMAccountName: "finance-approvers", Type: models.ADObjectTypeGroup},
							{DN: aliceDN, Name: "Alice", SAMAccountName: "alice", Type: models.ADObjectTypeUser},
						},
					},
					strings.ToLower(nestedDN): {
						DN:   nestedDN,
						Name: "Finance-Approvers",
						Members: []models.ADPrincipal{
							{DN: bobDN, Name: "Bob", SAMAccountName: "bob", Type: models.ADObjectTypeUser},
						},
					},
				},
			},
		}
		router := newTestRouter(applicationDependencies{ad: directoryService})

		recorder := performJSONRequestToRouter(t, router, "/api/ad/groups/members", ADGroupMembersRequest{
			ADCredentials: ADCredentials{
				Server:   "ldap://directory.example.com",
				BaseDN:   "DC=example,DC=com",
				Username: "svc-reader",
				Password: "secret",
			},
			GroupDN:       rootDN,
			IncludeNested: true,
			MaxDepth:      3,
		})

		assert.Equal(t, http.StatusOK, recorder.Code)
		body := recorder.Body.String()
		assert.Contains(t, body, `"resolution"`)
		assert.Contains(t, body, `"group_dn":"`+rootDN+`"`)
		assert.Contains(t, body, `"nested_groups":["`+nestedDN+`"]`)
		assert.Contains(t, body, `"sam_account_name":"bob"`)
		assert.Contains(t, body, `"max_depth_reached":false`)
	})

	t.Run("group client creation failure", func(t *testing.T) {
		directoryService := &stubADService{groupClientErr: errors.New("cannot connect")}
		router := newTestRouter(applicationDependencies{ad: directoryService})

		recorder := performJSONRequestToRouter(t, router, "/api/ad/groups/query", ADGroupQueryRequest{
			ADCredentials: ADCredentials{
				Server:   "ldap://directory.example.com",
				BaseDN:   "DC=example,DC=com",
				Username: "svc-reader",
				Password: "secret",
			},
			Query: "finance",
		})

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}

func TestADTreeEndpointHandlesSuccessAndFailure(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		directoryService := &stubADService{
			treeClient: &stubADTreeClient{
				nodes: []models.ADTreeNode{
					{DN: "OU=Finance,DC=example,DC=com", Name: "Finance", NodeType: "ou", HasChildren: true},
					{DN: "CN=FinanceTeam,OU=Finance,DC=example,DC=com", Name: "FinanceTeam", NodeType: "group", HasChildren: true},
					{DN: "CN=Policy1,CN=Policies,CN=System,DC=example,DC=com", Name: "Policy1", NodeType: "policy", HasChildren: false},
				},
			},
		}
		router := newTestRouter(applicationDependencies{ad: directoryService})

		recorder := performJSONRequestToRouter(t, router, "/api/ad/tree", ADTreeRequest{
			ADCredentials: ADCredentials{
				Server:   "ldap://directory.example.com",
				BaseDN:   "DC=example,DC=com",
				Username: "svc-reader",
				Password: "secret",
			},
			ParentDN:  "OU=Finance,DC=example,DC=com",
			Limit:     50,
			Page:      1,
			PageSize:  1,
			NodeTypes: []string{"group"},
		})

		assert.Equal(t, http.StatusOK, recorder.Code)
		require.NotNil(t, directoryService.treeClient)
		assert.Equal(t, "OU=Finance,DC=example,DC=com", directoryService.treeClient.lastParentDN)
		assert.Equal(t, 50, directoryService.treeClient.lastLimit)
		assert.True(t, directoryService.treeClient.closed)
		assert.Contains(t, recorder.Body.String(), `"total_count":1`)
		assert.Contains(t, recorder.Body.String(), `"node_type":"group"`)
	})

	t.Run("client creation failure", func(t *testing.T) {
		directoryService := &stubADService{treeClientErr: errors.New("cannot connect")}
		router := newTestRouter(applicationDependencies{ad: directoryService})

		recorder := performJSONRequestToRouter(t, router, "/api/ad/tree", ADTreeRequest{
			ADCredentials: ADCredentials{
				Server:   "ldap://directory.example.com",
				BaseDN:   "DC=example,DC=com",
				Username: "svc-reader",
				Password: "secret",
			},
		})

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}

func TestExportSummaryEndpointReturnsManagementView(t *testing.T) {
	router := newTestRouter(applicationDependencies{})
	recorder := performJSONRequestToRouter(t, router, "/api/export/summary", ExportSummaryRequest{
		Title:        "Q1 Audit Summary",
		Template:     "management",
		Organization: "Fold Security Team",
		PreparedBy:   "Ops Reviewer",
		ReportPeriod: "2026-Q1",
		FocusAreas:   []string{"High-risk rights", "Deny precedence"},
		Permissions: []models.Permission{
			{Path: `C:\Finance`, Trustee: `DOMAIN\Alice`, Rights: "Read", Type: "Allow", Inherited: false},
			{Path: `C:\Finance`, Trustee: `DOMAIN\Alice`, Rights: "Modify", Type: "Allow", Inherited: true},
			{Path: `C:\Finance\Payroll`, Trustee: `DOMAIN\Admins`, Rights: "Full Control", Type: "Deny", Inherited: false},
		},
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `"title":"Q1 Audit Summary"`)
	assert.Contains(t, body, `"total_permissions":3`)
	assert.Contains(t, body, `"high_risk_count":2`)
	assert.Contains(t, body, `"deny_count":1`)
	assert.Contains(t, body, `"template":"management"`)
	assert.Contains(t, body, `"organization":"Fold Security Team"`)
	assert.Contains(t, body, `"prepared_by":"Ops Reviewer"`)
	assert.Contains(t, body, `"report_period":"2026-Q1"`)
	assert.Contains(t, body, `"markdown":"# Q1 Audit Summary`)
}

func TestADTreeExplainEndpointReturnsHierarchy(t *testing.T) {
	router := newTestRouter(applicationDependencies{})
	recorder := performJSONRequestToRouter(t, router, "/api/ad/tree/explain", ADTreeExplainRequest{
		DN:       "OU=Finance,DC=example,DC=com",
		NodeType: "ou",
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `"node_type":"ou"`)
	assert.Contains(t, body, `"inheritance_chain"`)
	assert.Contains(t, body, `"risk_level":"`)
	assert.Contains(t, body, `"scope_targets"`)
	assert.Contains(t, body, `"recommended_checks"`)
	assert.Contains(t, body, `"complexity_score":`)
	assert.Contains(t, body, `"policy_hints"`)
	assert.Contains(t, body, `"delegation_boundaries"`)
	assert.Contains(t, body, "OU=Finance,DC=example,DC=com")
}

func TestExportSummaryEndpointSupportsComplianceTemplate(t *testing.T) {
	router := newTestRouter(applicationDependencies{})
	recorder := performJSONRequestToRouter(t, router, "/api/export/summary", ExportSummaryRequest{
		Template: "compliance",
		Permissions: []models.Permission{
			{Path: `C:\Finance`, Trustee: `DOMAIN\Alice`, Rights: "Read", Type: "Allow", Inherited: false},
			{Path: `C:\Finance\Payroll`, Trustee: `DOMAIN\Admins`, Rights: "Full Control", Type: "Deny", Inherited: false},
		},
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `"template":"compliance"`)
	assert.Contains(t, body, `"template_name":"Compliance"`)
	assert.Contains(t, body, "## Control Snapshot")
}

func TestExportSummaryEndpointSupportsSectionToggles(t *testing.T) {
	router := newTestRouter(applicationDependencies{})
	recorder := performJSONRequestToRouter(t, router, "/api/export/summary", ExportSummaryRequest{
		Template: "management",
		Sections: []string{"metadata", "kpis"},
		Permissions: []models.Permission{
			{Path: `C:\Finance`, Trustee: `DOMAIN\Alice`, Rights: "Read", Type: "Allow", Inherited: false},
			{Path: `C:\Finance\Payroll`, Trustee: `DOMAIN\Admins`, Rights: "Full Control", Type: "Deny", Inherited: false},
		},
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `"sections":["metadata","kpis"]`)
	assert.Contains(t, body, `- Total permissions: 2`)
	assert.NotContains(t, body, `## Top Trustees`)
	assert.NotContains(t, body, `## Top Paths`)
}

func TestExportSummaryTemplatesEndpointReturnsTemplates(t *testing.T) {
	router := newTestRouter(applicationDependencies{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/export/summary/templates", nil)
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `"count":3`)
	assert.Contains(t, body, `"id":"management"`)
	assert.Contains(t, body, `"id":"compliance"`)
	assert.Contains(t, body, `"id":"operations"`)
	assert.Contains(t, body, `"available_sections"`)
	assert.Contains(t, body, `"default_sections"`)
}

func TestPermissionConflictsEndpointReturnsConflictList(t *testing.T) {
	router := newTestRouter(applicationDependencies{})
	recorder := performJSONRequestToRouter(t, router, "/api/permissions/conflicts", PermissionConflictRequest{
		Permissions: []models.Permission{
			{Path: `C:\Finance`, Trustee: `DOMAIN\Alice`, Rights: "Read", Type: "Allow", Inherited: false},
			{Path: `C:\Finance`, Trustee: `DOMAIN\Alice`, Rights: "Read", Type: "Deny", Inherited: true},
			{Path: `C:\Finance`, Trustee: `DOMAIN\Bob`, Rights: "Read", Type: "Allow", Inherited: true},
		},
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `"count":1`)
	assert.Contains(t, body, `"trustee":"DOMAIN\\Alice"`)
	assert.Contains(t, body, `"precedence_note"`)
}

func TestAuditRequestsEndpointReturnsEntries(t *testing.T) {
	router := newTestRouter(applicationDependencies{})

	healthRecorder := httptest.NewRecorder()
	healthRequest := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(healthRecorder, healthRequest)
	require.Equal(t, http.StatusOK, healthRecorder.Code)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/audit/requests?page=1&page_size=20&path=health", nil)
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `"items"`)
	assert.Contains(t, body, `"pagination"`)
}

func TestAuditRequestDetailEndpointReturnsSingleRequest(t *testing.T) {
	router := newTestRouter(applicationDependencies{})

	healthRecorder := httptest.NewRecorder()
	healthRequest := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(healthRecorder, healthRequest)
	require.Equal(t, http.StatusOK, healthRecorder.Code)

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/audit/requests?page=1&page_size=1", nil)
	router.ServeHTTP(listRecorder, listRequest)
	require.Equal(t, http.StatusOK, listRecorder.Code)

	var listPayload struct {
		Items []auditEntry `json:"items"`
	}
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &listPayload))
	require.NotEmpty(t, listPayload.Items)

	requestID := listPayload.Items[0].RequestID
	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/audit/requests/"+requestID, nil)
	router.ServeHTTP(detailRecorder, detailRequest)

	assert.Equal(t, http.StatusOK, detailRecorder.Code)
	body := detailRecorder.Body.String()
	assert.Contains(t, body, `"request_id":"`+requestID+`"`)
	assert.Contains(t, body, `"entries"`)
	assert.Contains(t, body, `"latest"`)
}

func TestAuditRequestDetailEndpointSupportsPaginationAndMethodFilter(t *testing.T) {
	router := newTestRouter(applicationDependencies{})

	requestID := "audit-paged-req"
	for index := 0; index < 3; index++ {
		healthRecorder := httptest.NewRecorder()
		healthRequest := httptest.NewRequest(http.MethodGet, "/health", nil)
		healthRequest.Header.Set("X-Request-ID", requestID)
		router.ServeHTTP(healthRecorder, healthRequest)
		require.Equal(t, http.StatusOK, healthRecorder.Code)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/audit/requests/"+requestID+"?page=2&page_size=2&method=GET", nil)
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `"request_id":"`+requestID+`"`)
	assert.Contains(t, body, `"total_count":3`)
	assert.Contains(t, body, `"page":2`)
	assert.Contains(t, body, `"total_pages":2`)
}

func TestAuditSummaryExportEndpointReturnsMarkdown(t *testing.T) {
	router := newTestRouter(applicationDependencies{})

	healthRecorder := httptest.NewRecorder()
	healthRequest := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(healthRecorder, healthRequest)
	require.Equal(t, http.StatusOK, healthRecorder.Code)

	recorder := performJSONRequestToRouter(t, router, "/api/audit/requests/export/summary", AuditSummaryExportRequest{
		Path:  "health",
		Title: "Audit Export Check",
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/markdown; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Header().Get("Content-Disposition"), `attachment; filename="audit-summary.md"`)
	assert.Contains(t, recorder.Body.String(), "# Audit Export Check")
	assert.Contains(t, recorder.Body.String(), "## Top Endpoints")
}

type stubComparisonService struct {
	report      *comparison.ChangeReport
	err         error
	lastRequest *comparisonservice.Request
}

func (service *stubComparisonService) Compare(request comparisonservice.Request) (*comparison.ChangeReport, error) {
	service.lastRequest = &request
	return service.report, service.err
}

func (service *stubADService) NewConnectionClient(_ string, _ string, _ string, _ string) adConnectionClient {
	if service.connectionClient == nil {
		service.connectionClient = &stubADConnectionClient{}
	}

	return service.connectionClient
}

func (service *stubADService) NewUserSearchClient(_ string, _ string, _ string, _ string) (adUserSearchClient, error) {
	if service.userClientErr != nil {
		return nil, service.userClientErr
	}
	if service.userSearchClient == nil {
		service.userSearchClient = &stubADUserSearchClient{}
	}

	return service.userSearchClient, nil
}

func (service *stubADService) NewEffectivePermissionExpander(_ string, _ string, _ string, _ string, _ []string, _ []string) (scanservice.EffectivePermissionExpander, error) {
	service.permissionExpanderCalls++
	if service.permissionExpander != nil {
		return service.permissionExpander, nil
	}

	return nil, nil
}

func (service *stubADService) NewGroupClient(_ string, _ string, _ string, _ string) (adGroupClient, error) {
	if service.groupClientErr != nil {
		return nil, service.groupClientErr
	}
	if service.groupClient == nil {
		service.groupClient = &stubADGroupClient{}
	}

	return service.groupClient, nil
}

func (service *stubADService) NewTreeClient(_ string, _ string, _ string, _ string) (adTreeClient, error) {
	if service.treeClientErr != nil {
		return nil, service.treeClientErr
	}
	if service.treeClient == nil {
		service.treeClient = &stubADTreeClient{}
	}

	return service.treeClient, nil
}

type stubADConnectionClient struct {
	connected  bool
	closed     bool
	connectErr error
}

func (client *stubADConnectionClient) Connect() error {
	if client.connectErr != nil {
		return client.connectErr
	}

	client.connected = true
	return nil
}

func (client *stubADConnectionClient) Close() {
	client.closed = true
}

type stubADUserSearchClient struct {
	users     []ad.User
	searchErr error
	closed    bool
	lastQuery string
	lastLimit int
}

type stubADGroupClient struct {
	groups          []models.ADGroup
	group           *models.ADGroup
	groupsByDN      map[string]models.ADGroup
	principalsByDN  map[string]models.ADPrincipal
	searchErr       error
	getGroupErr     error
	getPrincipalErr error
	closed          bool
	lastQuery       string
	lastLimit       int
	lastRequestedDN string
	requestedDNs    []string
}

type stubADTreeClient struct {
	nodes        []models.ADTreeNode
	listErr      error
	closed       bool
	lastParentDN string
	lastLimit    int
}

type stubEffectivePermissionExpander struct {
	called              bool
	lastPermissions     []scanner.Permission
	expandedPermissions []scanner.Permission
}

func (expander *stubEffectivePermissionExpander) Expand(_ context.Context, permissions []scanner.Permission) ([]scanner.Permission, error) {
	expander.called = true
	expander.lastPermissions = append([]scanner.Permission(nil), permissions...)
	return append([]scanner.Permission(nil), expander.expandedPermissions...), nil
}

func (client *stubADUserSearchClient) SearchUsers(query string, limit int) ([]ad.User, error) {
	client.lastQuery = query
	client.lastLimit = limit
	if client.searchErr != nil {
		return nil, client.searchErr
	}

	return append([]ad.User(nil), client.users...), nil
}

func (client *stubADUserSearchClient) Close() {
	client.closed = true
}

func (client *stubADGroupClient) SearchGroups(query string, limit int) ([]models.ADGroup, error) {
	client.lastQuery = query
	client.lastLimit = limit
	if client.searchErr != nil {
		return nil, client.searchErr
	}

	return append([]models.ADGroup(nil), client.groups...), nil
}

func (client *stubADGroupClient) GetGroup(_ context.Context, distinguishedName string) (*models.ADGroup, error) {
	client.lastRequestedDN = distinguishedName
	client.requestedDNs = append(client.requestedDNs, distinguishedName)
	if client.getGroupErr != nil {
		return nil, client.getGroupErr
	}
	if client.groupsByDN != nil {
		if group, ok := client.groupsByDN[strings.ToLower(strings.TrimSpace(distinguishedName))]; ok {
			copyGroup := group
			copyGroup.Members = append([]models.ADPrincipal(nil), group.Members...)
			return &copyGroup, nil
		}

		return nil, nil
	}
	if client.group == nil {
		return &models.ADGroup{}, nil
	}

	copyGroup := *client.group
	copyGroup.Members = append([]models.ADPrincipal(nil), client.group.Members...)
	return &copyGroup, nil
}

func (client *stubADGroupClient) GetPrincipal(_ context.Context, distinguishedName string) (*models.ADPrincipal, error) {
	if client.getPrincipalErr != nil {
		return nil, client.getPrincipalErr
	}
	if client.principalsByDN != nil {
		if principal, ok := client.principalsByDN[strings.ToLower(strings.TrimSpace(distinguishedName))]; ok {
			copyPrincipal := principal
			return &copyPrincipal, nil
		}
	}

	return &models.ADPrincipal{}, nil
}

func (client *stubADGroupClient) ResolvePrincipal(_ context.Context, identifier string) (*models.ADPrincipal, error) {
	if client.getPrincipalErr != nil {
		return nil, client.getPrincipalErr
	}
	if client.principalsByDN != nil {
		if principal, ok := client.principalsByDN[strings.ToLower(strings.TrimSpace(identifier))]; ok {
			copyPrincipal := principal
			return &copyPrincipal, nil
		}
	}

	return nil, nil
}

func (client *stubADGroupClient) Close() {
	client.closed = true
}

func (client *stubADTreeClient) ListTreeNodes(_ context.Context, parentDN string, limit int) (ad.TreeListing, error) {
	client.lastParentDN = parentDN
	client.lastLimit = limit
	if client.listErr != nil {
		return ad.TreeListing{}, client.listErr
	}

	return ad.TreeListing{Nodes: append([]models.ADTreeNode(nil), client.nodes...)}, nil
}

func (client *stubADTreeClient) Close() {
	client.closed = true
}
