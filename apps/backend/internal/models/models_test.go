package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTableNames(t *testing.T) {
	assert.Equal(t, "scan_sessions", (ScanSession{}).TableName())
	assert.Equal(t, "permission_changes", (PermissionChange{}).TableName())
}

func TestBeforeCreateAssignsUUIDs(t *testing.T) {
	session := &ScanSession{}
	permission := &Permission{}
	change := &PermissionChange{}

	require.NoError(t, session.BeforeCreate(&gorm.DB{}))
	require.NoError(t, permission.BeforeCreate(&gorm.DB{}))
	require.NoError(t, change.BeforeCreate(&gorm.DB{}))

	assert.NotEqual(t, uuid.Nil, session.ID)
	assert.NotEqual(t, uuid.Nil, permission.ID)
	assert.NotEqual(t, uuid.Nil, change.ID)
}

func TestBeforeCreatePreservesExistingUUIDs(t *testing.T) {
	existingID := uuid.New()
	session := &ScanSession{ID: existingID}

	require.NoError(t, session.BeforeCreate(&gorm.DB{}))
	assert.Equal(t, existingID, session.ID)
}
