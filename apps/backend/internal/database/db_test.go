package database

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitWithDSNRequiresValue(t *testing.T) {
	previousDB := DB
	DB = nil
	t.Cleanup(func() {
		DB = previousDB
	})

	assert.EqualError(t, InitWithDSN("   "), "database dsn is required")
	assert.False(t, Ready())
}

func TestInitAndInitFromEnvUseDefaultSQLiteWhenDatabaseURLIsEmpty(t *testing.T) {
	previousDB := DB
	previousStoreDescription := StoreDescription
	dataDir := t.TempDir()
	DB = nil
	StoreDescription = ""
	t.Cleanup(func() {
		if DB != nil {
			if sqlDB, err := DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		DB = previousDB
		StoreDescription = previousStoreDescription
	})
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PERMISSION_PROTECTOR_DATA_DIR", dataDir)

	assert.NoError(t, Init())
	assert.True(t, Ready())
	assert.True(t, strings.HasPrefix(StoreDescription, "sqlite:"))
	assert.FileExists(t, filepath.Join(dataDir, "permission-protector.db"))
	assert.NoError(t, InitFromEnv())
}
