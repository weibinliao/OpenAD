package models

import (
	"time"
)

type Report struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // permission, user, share, owner
	RootPath    string    `json:"root_path"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relationships
	Sessions []ScanSession `json:"sessions" gorm:"foreignKey:ReportID"`
}

type ReportComparison struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	BaselineID   string    `json:"baseline_id"`
	CurrentID    string    `json:"current_id"`
	ChangesCount int       `json:"changes_count"`
	CreatedAt    time.Time `json:"created_at"`

	// Relationships
	Baseline ScanSession        `json:"baseline" gorm:"foreignKey:BaselineID"`
	Current  ScanSession        `json:"current" gorm:"foreignKey:CurrentID"`
	Changes  []PermissionChange `json:"changes" gorm:"foreignKey:ComparisonID"`
}
