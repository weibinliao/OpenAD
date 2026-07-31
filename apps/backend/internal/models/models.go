package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ScanSession struct {
	ID                        uuid.UUID          `gorm:"type:uuid;primaryKey" json:"id"`
	RootPath                  string             `gorm:"not null" json:"root_path"`
	Status                    string             `gorm:"not null;index" json:"status"`
	MaxDepth                  int                `gorm:"not null" json:"max_depth"`
	IncludeInherited          bool               `gorm:"not null" json:"include_inherited"`
	ItemsScanned              int                `gorm:"not null" json:"items_scanned"`
	PermissionCount           int                `gorm:"not null" json:"permission_count"`
	DirectorySyncRunID        *uuid.UUID         `gorm:"type:uuid;index" json:"directory_sync_run_id,omitempty"`
	IdentityResolutionMode    string             `json:"identity_resolution_mode,omitempty"`
	ResolvedPrincipalCount    int                `json:"resolved_principal_count"`
	UnresolvedPrincipalCount  int                `json:"unresolved_principal_count"`
	IdentityResolutionWarning string             `json:"identity_resolution_warning,omitempty"`
	ErrorMessage              string             `json:"error_message,omitempty"`
	StartedAt                 time.Time          `gorm:"not null;index:idx_scan_sessions_started_at,sort:desc" json:"started_at"`
	FinishedAt                *time.Time         `json:"finished_at,omitempty"`
	Permissions               []Permission       `gorm:"foreignKey:ScanSessionID" json:"permissions,omitempty"`
	Changes                   []PermissionChange `gorm:"foreignKey:ScanSessionID" json:"changes,omitempty"`
}

type Permission struct {
	ID                        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ScanSessionID             uuid.UUID `gorm:"type:uuid;not null;index;index:idx_permissions_session_sort,priority:1;index:idx_permissions_session_path,priority:1;index:idx_permissions_session_inherited,priority:1" json:"scan_session_id"`
	Path                      string    `gorm:"not null;index;index:idx_permissions_session_sort,priority:3;index:idx_permissions_session_path,priority:2" json:"path"`
	Trustee                   string    `gorm:"not null;index:idx_permissions_session_sort,priority:4" json:"trustee"`
	TrusteeSID                string    `gorm:"not null;index" json:"trustee_sid"`
	Rights                    string    `gorm:"not null" json:"rights"`
	Type                      string    `gorm:"not null" json:"type"`
	Inherited                 bool      `gorm:"not null;index;index:idx_permissions_session_inherited,priority:2" json:"inherited"`
	Source                    string    `json:"source,omitempty"`
	AppliesTo                 string    `json:"applies_to,omitempty"`
	AccountType               string    `json:"account_type,omitempty"`
	AccessMask                string    `json:"access_mask,omitempty"`
	RiskLevel                 string    `json:"risk_level,omitempty"`
	ParentDelta               string    `json:"parent_delta,omitempty"`
	AccountName               string    `json:"account_name,omitempty"`
	FirstName                 string    `json:"first_name,omitempty"`
	LastName                  string    `json:"last_name,omitempty"`
	Email                     string    `json:"email,omitempty"`
	Department                string    `json:"department,omitempty"`
	Division                  string    `json:"division,omitempty"`
	Domain                    string    `json:"domain,omitempty"`
	OriginatingGroup          string    `json:"originating_group,omitempty"`
	GroupInheritanceHierarchy string    `json:"group_inheritance_hierarchy,omitempty"`
	ResolutionSource          string    `json:"resolution_source,omitempty"`
	ResolutionReason          string    `json:"resolution_reason,omitempty"`
	CreatedAt                 time.Time `gorm:"index:idx_permissions_session_sort,priority:2,sort:desc" json:"created_at"`
}

type PermissionChange struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ScanSessionID  uuid.UUID `gorm:"type:uuid;not null;index;index:idx_permission_changes_session_sort,priority:1;index:idx_permission_changes_session_type,priority:1" json:"scan_session_id"`
	Path           string    `gorm:"not null;index;index:idx_permission_changes_session_sort,priority:3" json:"path"`
	Trustee        string    `gorm:"not null" json:"trustee"`
	TrusteeSID     string    `gorm:"not null;index;index:idx_permission_changes_session_sort,priority:4" json:"trustee_sid"`
	ChangeType     string    `gorm:"not null;index:idx_permission_changes_session_type,priority:2" json:"change_type"`
	PreviousRights string    `json:"previous_rights,omitempty"`
	CurrentRights  string    `json:"current_rights,omitempty"`
	DetectedAt     time.Time `gorm:"not null;index:idx_permission_changes_session_sort,priority:2,sort:desc" json:"detected_at"`
}

func (ScanSession) TableName() string {
	return "scan_sessions"
}

func (PermissionChange) TableName() string {
	return "permission_changes"
}

func (scan *ScanSession) BeforeCreate(_ *gorm.DB) error {
	if scan.ID == uuid.Nil {
		scan.ID = uuid.New()
	}

	return nil
}

func (permission *Permission) BeforeCreate(_ *gorm.DB) error {
	if permission.ID == uuid.Nil {
		permission.ID = uuid.New()
	}

	return nil
}

func (change *PermissionChange) BeforeCreate(_ *gorm.DB) error {
	if change.ID == uuid.Nil {
		change.ID = uuid.New()
	}

	return nil
}
