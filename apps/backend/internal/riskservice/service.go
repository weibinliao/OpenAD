// Package riskservice persists permission exposure findings and their review
// state in the OpenAD database.
package riskservice

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/weibinliao/OpenAD/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrDatabaseUnavailable = errors.New("database is not initialized")
	ErrInvalidInput        = errors.New("fingerprint, path, and trustee are required")
	ErrInvalidID           = errors.New("invalid risk finding id")
	ErrInvalidStatus       = errors.New("invalid risk finding status")
	ErrNotFound            = errors.New("risk finding not found")
)

const lookupBatchSize = 400

type FindingInput struct {
	Fingerprint       string    `json:"fingerprint"`
	Status            string    `json:"status,omitempty"`
	Severity          string    `json:"severity"`
	Type              string    `json:"type"`
	Title             string    `json:"title"`
	SuggestedAction   string    `json:"suggested_action"`
	Path              string    `json:"path"`
	Trustee           string    `json:"trustee"`
	TrusteeSID        string    `json:"trustee_sid"`
	Rights            string    `json:"rights"`
	Inherited         bool      `json:"inherited"`
	Source            string    `json:"source"`
	FirstSeenAt       time.Time `json:"first_seen_at"`
	LastSeenAt        time.Time `json:"last_seen_at"`
	LastSessionID     string    `json:"last_session_id,omitempty"`
	SeenCount         int       `json:"seen_count,omitempty"`
	Note              string    `json:"note,omitempty"`
	Description       string    `json:"description,omitempty"`
	Impact            string    `json:"impact,omitempty"`
	Category          string    `json:"category,omitempty"`
	PriorityScore     int       `json:"priority_score,omitempty"`
	Confidence        string    `json:"confidence,omitempty"`
	RemediationEffort string    `json:"remediation_effort,omitempty"`
	BusinessQuestion  string    `json:"business_question,omitempty"`
	ControlMapping    []string  `json:"control_mapping,omitempty"`
	Evidence          []string  `json:"evidence,omitempty"`
	SensitiveLabels   []string  `json:"sensitive_labels,omitempty"`
}

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (service *Service) List() ([]models.RiskFinding, error) {
	if err := service.ready(); err != nil {
		return nil, err
	}

	var items []models.RiskFinding
	err := service.db.Order("last_seen_at DESC, priority_score DESC, id ASC").Find(&items).Error
	return items, err
}

func (service *Service) UpsertFromScan(inputs []FindingInput) (int, error) {
	return service.merge(inputs, false)
}

func (service *Service) ImportLegacy(inputs []FindingInput) (int, error) {
	return service.merge(inputs, true)
}

func (service *Service) UpdateStatus(idValue, status string, note *string) (*models.RiskFinding, error) {
	if err := service.ready(); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(strings.TrimSpace(idValue))
	if err != nil {
		return nil, ErrInvalidID
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if !validStatus(status) {
		return nil, ErrInvalidStatus
	}

	var finding models.RiskFinding
	err = service.db.First(&finding, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	finding.Status = status
	if note != nil && strings.TrimSpace(*note) != "" {
		finding.Note = strings.TrimSpace(*note)
	}
	if err := service.db.Save(&finding).Error; err != nil {
		return nil, err
	}
	return &finding, nil
}

func (service *Service) merge(inputs []FindingInput, legacyImport bool) (int, error) {
	if err := service.ready(); err != nil {
		return 0, err
	}
	if len(inputs) == 0 {
		return 0, nil
	}

	normalized := make([]FindingInput, 0, len(inputs))
	inputByFingerprint := make(map[string]int, len(inputs))
	for _, raw := range inputs {
		input, err := normalizeInput(raw, legacyImport)
		if err != nil {
			return 0, err
		}
		if index, exists := inputByFingerprint[input.Fingerprint]; exists {
			normalized[index] = input
			continue
		}
		inputByFingerprint[input.Fingerprint] = len(normalized)
		normalized = append(normalized, input)
	}

	existingByFingerprint, err := service.loadExisting(normalized)
	if err != nil {
		return 0, err
	}

	items := make([]models.RiskFinding, 0, len(normalized))
	for _, input := range normalized {
		existing, found := existingByFingerprint[input.Fingerprint]
		if !found {
			items = append(items, newFinding(input, legacyImport))
			continue
		}

		if legacyImport {
			mergeLegacy(&existing, input)
		} else {
			mergeScan(&existing, input)
		}
		items = append(items, existing)
	}

	err = service.db.Transaction(func(tx *gorm.DB) error {
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "fingerprint"}},
			UpdateAll: true,
		}).CreateInBatches(&items, lookupBatchSize).Error
	})
	if err != nil {
		return 0, err
	}
	return len(normalized), nil
}

func (service *Service) loadExisting(inputs []FindingInput) (map[string]models.RiskFinding, error) {
	result := make(map[string]models.RiskFinding, len(inputs))
	fingerprints := make([]string, 0, len(inputs))
	for _, input := range inputs {
		fingerprints = append(fingerprints, input.Fingerprint)
	}

	for start := 0; start < len(fingerprints); start += lookupBatchSize {
		end := min(start+lookupBatchSize, len(fingerprints))
		var batch []models.RiskFinding
		if err := service.db.Where("fingerprint IN ?", fingerprints[start:end]).Find(&batch).Error; err != nil {
			return nil, err
		}
		for _, finding := range batch {
			result[finding.Fingerprint] = finding
		}
	}
	return result, nil
}

func (service *Service) ready() error {
	if service == nil || service.db == nil {
		return ErrDatabaseUnavailable
	}
	return nil
}

func normalizeInput(input FindingInput, legacyImport bool) (FindingInput, error) {
	input.Fingerprint = strings.TrimSpace(input.Fingerprint)
	input.Path = strings.TrimSpace(input.Path)
	input.Trustee = strings.TrimSpace(input.Trustee)
	if input.Fingerprint == "" || input.Path == "" || input.Trustee == "" {
		return FindingInput{}, ErrInvalidInput
	}

	now := time.Now().UTC()
	if input.FirstSeenAt.IsZero() {
		input.FirstSeenAt = now
	}
	if input.LastSeenAt.IsZero() {
		input.LastSeenAt = input.FirstSeenAt
	}
	if input.LastSeenAt.Before(input.FirstSeenAt) {
		input.FirstSeenAt = input.LastSeenAt
	}
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if !legacyImport || !validStatus(input.Status) {
		input.Status = "open"
	}
	input.Severity = normalizeSeverity(input.Severity)
	input.Type = valueOr(strings.TrimSpace(input.Type), "risk-finding")
	input.Title = valueOr(strings.TrimSpace(input.Title), "Permission risk requires review")
	input.SuggestedAction = strings.TrimSpace(input.SuggestedAction)
	input.TrusteeSID = strings.TrimSpace(input.TrusteeSID)
	input.Rights = strings.TrimSpace(input.Rights)
	input.Source = strings.TrimSpace(input.Source)
	input.LastSessionID = strings.TrimSpace(input.LastSessionID)
	input.SeenCount = max(1, input.SeenCount)
	input.Note = strings.TrimSpace(input.Note)
	input.Description = strings.TrimSpace(input.Description)
	input.Impact = strings.TrimSpace(input.Impact)
	input.Category = strings.TrimSpace(input.Category)
	input.PriorityScore = min(100, max(0, input.PriorityScore))
	input.Confidence = strings.TrimSpace(input.Confidence)
	input.RemediationEffort = strings.TrimSpace(input.RemediationEffort)
	input.BusinessQuestion = strings.TrimSpace(input.BusinessQuestion)
	input.ControlMapping = normalizeStrings(input.ControlMapping)
	input.Evidence = normalizeStrings(input.Evidence)
	input.SensitiveLabels = normalizeStrings(input.SensitiveLabels)
	return input, nil
}

func newFinding(input FindingInput, legacyImport bool) models.RiskFinding {
	status := "open"
	seenCount := 1
	if legacyImport {
		status = input.Status
		seenCount = input.SeenCount
	}
	finding := models.RiskFinding{
		ID:            uuid.New(),
		Fingerprint:   input.Fingerprint,
		Status:        status,
		FirstSeenAt:   input.FirstSeenAt,
		LastSeenAt:    input.LastSeenAt,
		LastSessionID: input.LastSessionID,
		SeenCount:     seenCount,
		Note:          input.Note,
	}
	applyDetails(&finding, input)
	return finding
}

func mergeScan(existing *models.RiskFinding, input FindingInput) {
	newObservation := input.LastSessionID == "" || existing.LastSessionID != input.LastSessionID
	if newObservation {
		existing.SeenCount++
		if existing.Status == "resolved" {
			existing.Status = "open"
		}
	}
	if input.FirstSeenAt.Before(existing.FirstSeenAt) {
		existing.FirstSeenAt = input.FirstSeenAt
	}
	if !input.LastSeenAt.Before(existing.LastSeenAt) {
		existing.LastSeenAt = input.LastSeenAt
		existing.LastSessionID = input.LastSessionID
		applyDetails(existing, input)
	}
}

func mergeLegacy(existing *models.RiskFinding, input FindingInput) {
	if input.FirstSeenAt.Before(existing.FirstSeenAt) {
		existing.FirstSeenAt = input.FirstSeenAt
	}
	if input.LastSeenAt.After(existing.LastSeenAt) {
		existing.LastSeenAt = input.LastSeenAt
		existing.LastSessionID = input.LastSessionID
		applyDetails(existing, input)
	}
	existing.SeenCount = max(existing.SeenCount, input.SeenCount)
}

func applyDetails(finding *models.RiskFinding, input FindingInput) {
	finding.Severity = input.Severity
	finding.Type = input.Type
	finding.Title = input.Title
	finding.SuggestedAction = input.SuggestedAction
	finding.Path = input.Path
	finding.Trustee = input.Trustee
	finding.TrusteeSID = input.TrusteeSID
	finding.Rights = input.Rights
	finding.Inherited = input.Inherited
	finding.Source = input.Source
	finding.Description = input.Description
	finding.Impact = input.Impact
	finding.Category = input.Category
	finding.PriorityScore = input.PriorityScore
	finding.Confidence = input.Confidence
	finding.RemediationEffort = input.RemediationEffort
	finding.BusinessQuestion = input.BusinessQuestion
	finding.ControlMapping = input.ControlMapping
	finding.Evidence = input.Evidence
	finding.SensitiveLabels = input.SensitiveLabels
}

func validStatus(status string) bool {
	return status == "open" || status == "accepted" || status == "resolved"
}

func normalizeSeverity(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "critical" || value == "high" || value == "medium" || value == "low" {
		return value
	}
	return "medium"
}

func normalizeStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
