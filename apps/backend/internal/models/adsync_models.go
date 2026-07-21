package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DirectorySyncRun tracks one snapshot of AD users, groups and flattened
// memberships taken through a stored connection profile.
type DirectorySyncRun struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	ConnectionID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"connection_id"`
	Status          string     `gorm:"not null;index" json:"status"` // running | completed | failed
	UserCount       int        `gorm:"not null;default:0" json:"user_count"`
	GroupCount      int        `gorm:"not null;default:0" json:"group_count"`
	MembershipCount int        `gorm:"not null;default:0" json:"membership_count"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	StartedAt       time.Time  `gorm:"not null;index:idx_directory_sync_runs_started_at,sort:desc" json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

// ADUserRecord is one snapshotted AD user belonging to a sync run.
type ADUserRecord struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	RunID          uuid.UUID `gorm:"type:uuid;not null;index;index:idx_ad_users_run_sid,priority:1" json:"run_id"`
	SID            string    `gorm:"column:sid;not null;index:idx_ad_users_run_sid,priority:2" json:"sid"`
	SAMAccountName string    `json:"sam_account_name"`
	UPN            string    `json:"upn"`
	DisplayName    string    `json:"display_name"`
	Email          string    `json:"email"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Department     string    `json:"department"`
	Division       string    `json:"division"`
	Domain         string    `json:"domain"`
	DN             string    `json:"dn"`
	Enabled        bool      `gorm:"not null;default:true" json:"enabled"`
}

// ADGroupRecord is one snapshotted AD group belonging to a sync run.
type ADGroupRecord struct {
	ID    uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	RunID uuid.UUID `gorm:"type:uuid;not null;index;index:idx_ad_groups_run_sid,priority:1" json:"run_id"`
	SID   string    `gorm:"column:sid;not null;index:idx_ad_groups_run_sid,priority:2" json:"sid"`
	Name  string    `json:"name"`
	DN    string    `json:"dn"`
	Scope string    `json:"scope"` // global | domainlocal | universal | ""
}

// ADMembershipRecord is one flattened membership edge (user->group or
// group->group, direct or transitive) belonging to a sync run.
type ADMembershipRecord struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	RunID     uuid.UUID `gorm:"type:uuid;not null;index:idx_ad_memberships_run_member,priority:1;index:idx_ad_memberships_run_group,priority:1" json:"run_id"`
	MemberSID string    `gorm:"column:member_sid;not null;index:idx_ad_memberships_run_member,priority:2" json:"member_sid"`
	GroupSID  string    `gorm:"column:group_sid;not null;index:idx_ad_memberships_run_group,priority:2" json:"group_sid"`
	Direct    bool      `gorm:"not null" json:"direct"`
	ViaChain  string    `json:"via_chain,omitempty"` // e.g. "Sales-Team > All-Staff"
}

func (DirectorySyncRun) TableName() string {
	return "directory_sync_runs"
}

func (ADUserRecord) TableName() string {
	return "ad_users"
}

func (ADGroupRecord) TableName() string {
	return "ad_groups"
}

func (ADMembershipRecord) TableName() string {
	return "ad_memberships"
}

func (run *DirectorySyncRun) BeforeCreate(_ *gorm.DB) error {
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}

	return nil
}
