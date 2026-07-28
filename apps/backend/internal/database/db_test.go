package database

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestInitWithDSNAppliesSQLiteConcurrencyConfiguration(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "concurrency.db")
	resetDatabaseForTest(t)

	require.NoError(t, InitWithDSN("sqlite://"+databasePath))

	var journalMode string
	require.NoError(t, DB.Raw("PRAGMA journal_mode").Scan(&journalMode).Error)
	assert.Equal(t, "wal", strings.ToLower(journalMode))

	var busyTimeout int
	require.NoError(t, DB.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error)
	assert.Equal(t, 5000, busyTimeout)

	sqlDB, err := DB.DB()
	require.NoError(t, err)
	assert.Equal(t, 1, sqlDB.Stats().MaxOpenConnections)
}

func TestSQLiteWriteWaitsForExistingWriter(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "writer-lock.db")
	resetDatabaseForTest(t)
	require.NoError(t, InitWithDSN("sqlite://"+databasePath))
	require.NoError(t, DB.Exec("CREATE TABLE lock_probe (value TEXT)").Error)

	locker, err := sql.Open("sqlite", databasePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = locker.Close() })

	transaction, err := locker.Begin()
	require.NoError(t, err)
	defer func() { _ = transaction.Rollback() }()
	_, err = transaction.Exec("INSERT INTO lock_probe (value) VALUES ('locker')")
	require.NoError(t, err)

	writeDone := make(chan error, 1)
	startedAt := time.Now()
	go func() {
		writeDone <- DB.Exec("INSERT INTO lock_probe (value) VALUES ('waiting writer')").Error
	}()

	select {
	case writeErr := <-writeDone:
		require.NoError(t, writeErr)
		t.Fatal("write completed before the existing writer released its lock")
	case <-time.After(150 * time.Millisecond):
	}

	require.NoError(t, transaction.Commit())
	select {
	case writeErr := <-writeDone:
		require.NoError(t, writeErr)
	case <-time.After(6 * time.Second):
		t.Fatal("waiting writer did not complete within busy_timeout plus tolerance")
	}
	assert.GreaterOrEqual(t, time.Since(startedAt), 150*time.Millisecond)
}

func resetDatabaseForTest(t *testing.T) {
	t.Helper()
	previousDB := DB
	previousStoreDescription := StoreDescription
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
}
