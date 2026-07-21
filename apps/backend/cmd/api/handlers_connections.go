package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/weibinliao/OpenAD/internal/ad"
	"github.com/weibinliao/OpenAD/internal/connections"
	"github.com/weibinliao/OpenAD/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ADDiscoverRequest probes a domain controller with just server + account +
// password; Base DN, DNS domain, and the normalized bind identity come back.
type ADDiscoverRequest struct {
	Server   string `json:"server" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (application *application) handleADDiscover(context *gin.Context) {
	var request ADDiscoverRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := ad.Discover(request.Server, request.Username, request.Password)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	context.JSON(http.StatusOK, result)
}

// autoDiscoverProfileInput fills BaseDN (and normalizes the bind user) when
// the operator only supplied server + account + password.
func autoDiscoverProfileInput(input *connections.ProfileInput) error {
	if strings.TrimSpace(input.BaseDN) != "" {
		return nil
	}
	if input.Password == "" {
		return errors.New("base_dn is empty and no password was provided to auto-discover it")
	}
	result, err := ad.Discover(input.Server, input.BindUser, input.Password)
	if err != nil {
		return err
	}
	input.BaseDN = result.BaseDN
	input.BindUser = result.BindUser
	return nil
}

// connectionsService returns the stored-connection service when persistence
// is available; endpoints degrade with 503 otherwise.
func connectionsService() *connections.Service {
	if !database.Ready() {
		return nil
	}
	return connections.NewService(database.DB, database.DataDir())
}

func respondConnectionsUnavailable(context *gin.Context) {
	context.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not initialized; stored connections are unavailable"})
}

func handleConnectionServiceError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, connections.ErrNotFound):
		context.JSON(http.StatusNotFound, gin.H{"error": "connection profile not found"})
	case errors.Is(err, connections.ErrNameRequired), errors.Is(err, connections.ErrInvalidInput):
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (application *application) handleListADConnections(context *gin.Context) {
	service := connectionsService()
	if service == nil {
		respondConnectionsUnavailable(context)
		return
	}

	profiles, err := service.List()
	if err != nil {
		handleConnectionServiceError(context, err)
		return
	}
	context.JSON(http.StatusOK, gin.H{"items": profiles, "count": len(profiles)})
}

func (application *application) handleCreateADConnection(context *gin.Context) {
	service := connectionsService()
	if service == nil {
		respondConnectionsUnavailable(context)
		return
	}

	var input connections.ProfileInput
	if err := context.ShouldBindJSON(&input); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := autoDiscoverProfileInput(&input); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile, err := service.Create(input)
	if err != nil {
		handleConnectionServiceError(context, err)
		return
	}
	context.JSON(http.StatusCreated, profile)
}

func (application *application) handleUpdateADConnection(context *gin.Context) {
	service := connectionsService()
	if service == nil {
		respondConnectionsUnavailable(context)
		return
	}

	id, err := uuid.Parse(context.Param("id"))
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid connection id"})
		return
	}

	var input connections.ProfileInput
	if err := context.ShouldBindJSON(&input); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile, err := service.Update(id, input)
	if err != nil {
		handleConnectionServiceError(context, err)
		return
	}
	context.JSON(http.StatusOK, profile)
}

func (application *application) handleDeleteADConnection(context *gin.Context) {
	service := connectionsService()
	if service == nil {
		respondConnectionsUnavailable(context)
		return
	}

	id, err := uuid.Parse(context.Param("id"))
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid connection id"})
		return
	}

	if err := service.Delete(id); err != nil {
		handleConnectionServiceError(context, err)
		return
	}
	context.JSON(http.StatusOK, gin.H{"message": "connection profile deleted"})
}

func (application *application) handleTestADConnection(context *gin.Context) {
	service := connectionsService()
	if service == nil {
		respondConnectionsUnavailable(context)
		return
	}

	id, err := uuid.Parse(context.Param("id"))
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid connection id"})
		return
	}

	credentials, err := service.Resolve(id)
	if err != nil {
		handleConnectionServiceError(context, err)
		return
	}

	client := application.ad.NewConnectionClient(credentials.Server, credentials.BaseDN, credentials.BindUser, credentials.Password)
	connectErr := client.Connect()
	if connectErr == nil {
		defer client.Close()
	}
	_ = service.RecordTestResult(id, connectErr == nil)

	if connectErr != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "AD connection failed: " + connectErr.Error(), "ok": false})
		return
	}
	context.JSON(http.StatusOK, gin.H{"message": "Active Directory connection successful", "ok": true})
}

// ADCredentials is embedded by AD request types. Callers provide either a
// stored connection_id or full inline credentials; resolveInto fills the
// inline fields server-side so downstream code keeps working unchanged.
type ADCredentials struct {
	ConnectionID string `json:"connection_id"`
	Server       string `json:"server"`
	BaseDN       string `json:"base_dn"`
	Username     string `json:"username"`
	Password     string `json:"password"`
}

func (credentials *ADCredentials) resolveInto() error {
	server, baseDN, username, password, err := resolveADCredentials(
		credentials.ConnectionID, credentials.Server, credentials.BaseDN, credentials.Username, credentials.Password)
	if err != nil {
		return err
	}
	credentials.Server, credentials.BaseDN, credentials.Username, credentials.Password = server, baseDN, username, password
	return nil
}

// resolveADCredentials lets AD endpoints accept either a stored connection_id
// or inline credentials (legacy callers). Inline credentials win when complete.
func resolveADCredentials(connectionID, server, baseDN, username, password string) (string, string, string, string, error) {
	inlineComplete := strings.TrimSpace(server) != "" && strings.TrimSpace(baseDN) != "" &&
		strings.TrimSpace(username) != "" && password != ""

	if strings.TrimSpace(connectionID) != "" {
		service := connectionsService()
		if service == nil {
			return "", "", "", "", errors.New("database is not initialized; stored connections are unavailable")
		}
		id, err := uuid.Parse(strings.TrimSpace(connectionID))
		if err != nil {
			return "", "", "", "", errors.New("invalid connection_id")
		}
		credentials, err := service.Resolve(id)
		if err != nil {
			return "", "", "", "", err
		}
		return credentials.Server, credentials.BaseDN, credentials.BindUser, credentials.Password, nil
	}

	if inlineComplete {
		return server, baseDN, username, password, nil
	}

	return "", "", "", "", errors.New("connection_id or full inline credentials (server, base_dn, username, password) are required")
}
