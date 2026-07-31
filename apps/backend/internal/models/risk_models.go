package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RiskFinding stores the durable review state and evidence generated from
// completed permission scans. Large finding sets belong in the application
// database rather than the browser's quota-limited localStorage.
type RiskFinding struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Fingerprint       string    `gorm:"not null;uniqueIndex" json:"fingerprint"`
	Status            string    `gorm:"not null;index" json:"status"`
	Severity          string    `gorm:"not null;index" json:"severity"`
	Type              string    `gorm:"not null;index" json:"type"`
	Title             string    `gorm:"type:text;not null" json:"title"`
	SuggestedAction   string    `gorm:"type:text" json:"suggested_action"`
	Path              string    `gorm:"type:text;not null" json:"path"`
	Trustee           string    `gorm:"type:text;not null" json:"trustee"`
	TrusteeSID        string    `gorm:"index" json:"trustee_sid"`
	Rights            string    `gorm:"type:text" json:"rights"`
	Inherited         bool      `gorm:"not null" json:"inherited"`
	Source            string    `json:"source"`
	FirstSeenAt       time.Time `gorm:"not null" json:"first_seen_at"`
	LastSeenAt        time.Time `gorm:"not null;index:idx_risk_findings_last_seen,sort:desc" json:"last_seen_at"`
	LastSessionID     string    `gorm:"index" json:"last_session_id,omitempty"`
	SeenCount         int       `gorm:"not null" json:"seen_count"`
	Note              string    `gorm:"type:text" json:"note,omitempty"`
	Description       string    `gorm:"type:text" json:"description,omitempty"`
	Impact            string    `gorm:"type:text" json:"impact,omitempty"`
	Category          string    `gorm:"index" json:"category,omitempty"`
	PriorityScore     int       `gorm:"index" json:"priority_score,omitempty"`
	Confidence        string    `json:"confidence,omitempty"`
	RemediationEffort string    `json:"remediation_effort,omitempty"`
	BusinessQuestion  string    `gorm:"type:text" json:"business_question,omitempty"`
	ControlMapping    []string  `gorm:"serializer:json" json:"control_mapping,omitempty"`
	Evidence          []string  `gorm:"serializer:json" json:"evidence,omitempty"`
	SensitiveLabels   []string  `gorm:"serializer:json" json:"sensitive_labels,omitempty"`
	CreatedAt         time.Time `json:"-"`
	UpdatedAt         time.Time `json:"-"`
}

func (RiskFinding) TableName() string {
	return "risk_findings"
}

func (finding *RiskFinding) BeforeCreate(_ *gorm.DB) error {
	if finding.ID == uuid.Nil {
		finding.ID = uuid.New()
	}
	return nil
}
