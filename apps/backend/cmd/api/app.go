package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/weibinliao/OpenAD/internal/ad"
	"github.com/weibinliao/OpenAD/internal/comparison"
	"github.com/weibinliao/OpenAD/internal/comparisonservice"
	"github.com/weibinliao/OpenAD/internal/database"
	"github.com/weibinliao/OpenAD/internal/export"
	"github.com/weibinliao/OpenAD/internal/historyservice"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/riskservice"
	"github.com/weibinliao/OpenAD/internal/scanservice"
)

type scanRunner interface {
	Run(request scanservice.Request) (*scanservice.Response, error)
}

type historyReader interface {
	ListSessions(filter historyservice.SessionListFilter) (*historyservice.SessionListResponse, error)
	GetSession(id string) (*models.ScanSession, error)
	GetSessionBundle(id string) (*historyservice.SessionBundleResponse, error)
	ListSessionPermissions(id string, filter historyservice.PermissionListFilter) (*historyservice.PermissionListResponse, error)
	ListSessionChanges(id string, filter historyservice.ChangeListFilter) (*historyservice.ChangeListResponse, error)
}

type comparisonRunner interface {
	Compare(request comparisonservice.Request) (*comparison.ChangeReport, error)
}

type riskFindingStore interface {
	List() ([]models.RiskFinding, error)
	UpsertFromScan(inputs []riskservice.FindingInput) (int, error)
	ImportLegacy(inputs []riskservice.FindingInput) (int, error)
	UpdateStatus(id, status string, note *string) (*models.RiskFinding, error)
}

type fileExporter interface {
	ExportToCSV(permissions []models.Permission, filename string, options export.Options) error
	ExportToExcel(permissions []models.Permission, filename string, options export.Options) error
	ExportToHTML(permissions []models.Permission, filename string, options export.Options) error
}

type adConnectionClient interface {
	Connect() error
	Close()
}

type adUserSearchClient interface {
	SearchUsers(query string, limit int) ([]ad.User, error)
	Close()
}

type adGroupClient interface {
	SearchGroups(query string, limit int) ([]models.ADGroup, error)
	GetGroup(ctx context.Context, distinguishedName string) (*models.ADGroup, error)
	GetPrincipal(ctx context.Context, distinguishedName string) (*models.ADPrincipal, error)
	ResolvePrincipal(ctx context.Context, identifier string) (*models.ADPrincipal, error)
	Close()
}

type adTreeClient interface {
	ListTreeNodes(ctx context.Context, parentDN string, limit int) (ad.TreeListing, error)
	Close()
}

type activeDirectoryService interface {
	NewConnectionClient(server, baseDN, username, password string) adConnectionClient
	NewUserSearchClient(server, baseDN, username, password string) (adUserSearchClient, error)
	NewGroupClient(server, baseDN, username, password string) (adGroupClient, error)
	NewTreeClient(server, baseDN, username, password string) (adTreeClient, error)
	NewEffectivePermissionExpander(server, baseDN, username, password string, excludeGroupPatterns, excludeUserPatterns []string) (scanservice.EffectivePermissionExpander, error)
}

type applicationDependencies struct {
	scans        scanRunner
	history      historyReader
	comparison   comparisonRunner
	riskFindings riskFindingStore
	exporter     fileExporter
	ad           activeDirectoryService
	progressHub  *scanProgressHub
	scanCancels  *scanCancelRegistry
}

type application struct {
	scans        scanRunner
	history      historyReader
	comparison   comparisonRunner
	riskFindings riskFindingStore
	exporter     fileExporter
	ad           activeDirectoryService
	progressHub  *scanProgressHub
	scanCancels  *scanCancelRegistry
}

type defaultADService struct{}

func newApplication(dependencies applicationDependencies) *application {
	if dependencies.scans == nil {
		dependencies.scans = scanservice.New()
	}

	if dependencies.history == nil {
		dependencies.history = historyservice.New()
	}

	if dependencies.comparison == nil {
		dependencies.comparison = comparisonservice.New()
	}

	if dependencies.riskFindings == nil {
		dependencies.riskFindings = riskservice.New(database.DB)
	}

	if dependencies.exporter == nil {
		dependencies.exporter = export.NewExporter()
	}

	if dependencies.ad == nil {
		dependencies.ad = defaultADService{}
	}

	if dependencies.progressHub == nil {
		dependencies.progressHub = newScanProgressHub()
	}

	if dependencies.scanCancels == nil {
		dependencies.scanCancels = newScanCancelRegistry()
	}

	return &application{
		scans:        dependencies.scans,
		history:      dependencies.history,
		comparison:   dependencies.comparison,
		riskFindings: dependencies.riskFindings,
		exporter:     dependencies.exporter,
		ad:           dependencies.ad,
		progressHub:  dependencies.progressHub,
		scanCancels:  dependencies.scanCancels,
	}
}

func dedupeNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		key := strings.ToLower(trimmed)
		if _, found := seen[key]; found {
			continue
		}

		seen[key] = struct{}{}
		result = append(result, trimmed)
	}

	return result
}

func parsePositiveIntQuery(context *gin.Context, key string) (int, error) {
	rawValue := context.Query(key)
	if rawValue == "" {
		return 0, nil
	}

	value, err := strconv.Atoi(rawValue)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("invalid %s", key)
	}

	return value, nil
}

func parseBoolQuery(context *gin.Context, key string) (*bool, error) {
	rawValue := context.Query(key)
	if rawValue == "" {
		return nil, nil
	}

	value, err := strconv.ParseBool(rawValue)
	if err != nil {
		return nil, fmt.Errorf("invalid %s", key)
	}

	return &value, nil
}

func handleHistoryError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, historyservice.ErrInvalidSessionID):
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid session id"})
	case errors.Is(err, historyservice.ErrSessionNotFound):
		context.JSON(http.StatusNotFound, gin.H{"error": "scan session not found"})
	case errors.Is(err, historyservice.ErrDatabaseUnavailable):
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not initialized"})
	default:
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (defaultADService) NewConnectionClient(server, baseDN, username, password string) adConnectionClient {
	return ad.NewClient(server, baseDN, username, password)
}

func (defaultADService) NewUserSearchClient(server, baseDN, username, password string) (adUserSearchClient, error) {
	return ad.NewADClient(server, baseDN, username, password)
}

func (defaultADService) NewGroupClient(server, baseDN, username, password string) (adGroupClient, error) {
	return ad.NewADClient(server, baseDN, username, password)
}

func (defaultADService) NewTreeClient(server, baseDN, username, password string) (adTreeClient, error) {
	return ad.NewADClient(server, baseDN, username, password)
}

func (defaultADService) NewEffectivePermissionExpander(server, baseDN, username, password string, excludeGroupPatterns, excludeUserPatterns []string) (scanservice.EffectivePermissionExpander, error) {
	client, err := ad.NewADClient(server, baseDN, username, password)
	if err != nil {
		return nil, err
	}

	return ad.NewPermissionExpander(
		client,
		client,
		ad.WithPermissionExclusionPatterns(excludeGroupPatterns, excludeUserPatterns),
	), nil
}
