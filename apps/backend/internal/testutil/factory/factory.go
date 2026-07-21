package factory

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/weibinliao/OpenAD/internal/ad"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/scanner"
	"github.com/weibinliao/OpenAD/internal/scanservice"
	"github.com/google/uuid"
)

type ScanSessionParams struct {
	ID               uuid.UUID
	RootPath         string
	Status           string
	MaxDepth         int
	IncludeInherited bool
	ItemsScanned     int
	PermissionCount  int
	StartedAt        time.Time
	FinishedAt       *time.Time
	ErrorMessage     string
}

type PermissionParams struct {
	ID            uuid.UUID
	ScanSessionID uuid.UUID
	Path          string
	Trustee       string
	TrusteeSID    string
	Rights        string
	Type          string
	Inherited     bool
	Source        string
	CreatedAt     time.Time
}

type ScanResponseParams struct {
	SessionID        string
	RootPath         string
	MaxDepth         int
	IncludeInherited bool
	ItemsScanned     int
	PermissionCount  int
	StartedAt        time.Time
	FinishedAt       time.Time
	Permissions      []scanner.Permission
	Skipped          []scanner.PathError
}

type ADUserParams struct {
	DN          string
	Username    string
	DisplayName string
	Email       string
	Groups      []string
}

var sequence uint64

func NewScanSession(params ScanSessionParams) models.ScanSession {
	index := nextSequence()
	startedAt := params.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}

	rootPath := params.RootPath
	if rootPath == "" {
		rootPath = fmt.Sprintf(`C:\Factory\Root\%d`, index)
	}

	status := params.Status
	if status == "" {
		status = "completed"
	}

	maxDepth := params.MaxDepth
	if maxDepth == 0 {
		maxDepth = 3
	}

	permissionCount := params.PermissionCount
	if permissionCount == 0 {
		permissionCount = 1
	}

	itemsScanned := params.ItemsScanned
	if itemsScanned == 0 {
		itemsScanned = permissionCount
	}

	id := params.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	return models.ScanSession{
		ID:               id,
		RootPath:         rootPath,
		Status:           status,
		MaxDepth:         maxDepth,
		IncludeInherited: params.IncludeInherited,
		ItemsScanned:     itemsScanned,
		PermissionCount:  permissionCount,
		ErrorMessage:     params.ErrorMessage,
		StartedAt:        startedAt,
		FinishedAt:       params.FinishedAt,
	}
}

func NewPermission(params PermissionParams) models.Permission {
	index := nextSequence()
	createdAt := params.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	path := params.Path
	if path == "" {
		path = fmt.Sprintf(`C:\Factory\Root\%d`, index)
	}

	trustee := params.Trustee
	if trustee == "" {
		trustee = fmt.Sprintf(`DOMAIN\User%02d`, index)
	}

	trusteeSID := params.TrusteeSID
	if trusteeSID == "" {
		trusteeSID = fmt.Sprintf("S-1-5-21-%d", index)
	}

	rights := params.Rights
	if rights == "" {
		rights = "Read"
	}

	permissionType := params.Type
	if permissionType == "" {
		permissionType = "Allow"
	}

	id := params.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	return models.Permission{
		ID:            id,
		ScanSessionID: params.ScanSessionID,
		Path:          path,
		Trustee:       trustee,
		TrusteeSID:    trusteeSID,
		Rights:        rights,
		Type:          permissionType,
		Inherited:     params.Inherited,
		Source:        params.Source,
		CreatedAt:     createdAt,
	}
}

func NewScanResponse(params ScanResponseParams) scanservice.Response {
	index := nextSequence()
	startedAt := params.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().Add(-1 * time.Minute).UTC()
	}

	finishedAt := params.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}

	rootPath := params.RootPath
	if rootPath == "" {
		rootPath = fmt.Sprintf(`C:\Factory\Scan\%d`, index)
	}

	maxDepth := params.MaxDepth
	if maxDepth == 0 {
		maxDepth = 3
	}

	permissionCount := params.PermissionCount
	if permissionCount == 0 {
		permissionCount = len(params.Permissions)
	}
	if permissionCount == 0 {
		permissionCount = 1
	}

	itemsScanned := params.ItemsScanned
	if itemsScanned == 0 {
		itemsScanned = permissionCount
	}

	sessionID := params.SessionID
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	permissions := append([]scanner.Permission(nil), params.Permissions...)
	if len(permissions) == 0 {
		permissions = []scanner.Permission{{
			Path:       rootPath,
			Trustee:    `DOMAIN\FactoryUser`,
			TrusteeSID: "S-1-5-21-500",
			Rights:     "Read",
			Type:       "Allow",
		}}
	}

	return scanservice.Response{
		SessionID:        sessionID,
		RootPath:         rootPath,
		MaxDepth:         maxDepth,
		IncludeInherited: params.IncludeInherited,
		ItemsScanned:     itemsScanned,
		PermissionCount:  permissionCount,
		StartedAt:        startedAt,
		FinishedAt:       finishedAt,
		Permissions:      permissions,
		Skipped:          append([]scanner.PathError(nil), params.Skipped...),
	}
}

func NewADUser(params ADUserParams) ad.User {
	index := nextSequence()
	distinguishedName := params.DN
	if distinguishedName == "" {
		distinguishedName = fmt.Sprintf("CN=Factory User %d,OU=Users,DC=example,DC=com", index)
	}

	username := params.Username
	if username == "" {
		username = fmt.Sprintf("factory-user-%d", index)
	}

	displayName := params.DisplayName
	if displayName == "" {
		displayName = fmt.Sprintf("Factory User %d", index)
	}

	email := params.Email
	if email == "" {
		email = fmt.Sprintf("factory-user-%d@example.com", index)
	}

	return ad.User{
		DN:          distinguishedName,
		Username:    username,
		DisplayName: displayName,
		Email:       email,
		Groups:      append([]string(nil), params.Groups...),
	}
}

func nextSequence() uint64 {
	return atomic.AddUint64(&sequence, 1)
}
