package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ADConnectionProfile is a stored, reusable Active Directory connection.
// The bind password is encrypted at rest (internal/secrets) and is never
// serialized into API responses.
type ADConnectionProfile struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name              string    `gorm:"not null;uniqueIndex" json:"name"`
	Server            string    `gorm:"not null" json:"server"`
	BaseDN            string    `gorm:"not null" json:"base_dn"`
	BindUser          string    `gorm:"not null" json:"bind_user"`
	EncryptedPassword string    `gorm:"not null" json:"-"`
	IsDefault         bool      `gorm:"not null;default:false" json:"is_default"`
	LastTestedAt      *time.Time `json:"last_tested_at,omitempty"`
	LastTestOK        *bool      `json:"last_test_ok,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (ADConnectionProfile) TableName() string {
	return "ad_connection_profiles"
}

func (profile *ADConnectionProfile) BeforeCreate(_ *gorm.DB) error {
	if profile.ID == uuid.Nil {
		profile.ID = uuid.New()
	}
	return nil
}
