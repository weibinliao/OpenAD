package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveTestDatabaseURLUsesExplicitURL(t *testing.T) {
	t.Setenv("TEST_DATABASE_URL", "postgres://custom:secret@db:5434/custom_db?sslmode=disable")

	assert.Equal(t, "postgres://custom:secret@db:5434/custom_db?sslmode=disable", ResolveTestDatabaseURL())
}

func TestResolveTestDatabaseURLBuildsURLFromDefaults(t *testing.T) {
	t.Setenv("TEST_DATABASE_URL", "")
	t.Setenv("TEST_DATABASE_HOST", "")
	t.Setenv("TEST_DATABASE_PORT", "")
	t.Setenv("TEST_DATABASE_NAME", "")
	t.Setenv("TEST_DATABASE_USER", "")
	t.Setenv("TEST_DATABASE_PASSWORD", "")
	t.Setenv("TEST_DATABASE_SSLMODE", "")

	assert.Equal(t, "postgres://test:test@127.0.0.1:5433/folder_security_test?sslmode=disable", ResolveTestDatabaseURL())
}

func TestDefaultDatabaseConfigHonorsEnvironmentOverrides(t *testing.T) {
	t.Setenv("TEST_DATABASE_HOST", "db")
	t.Setenv("TEST_DATABASE_PORT", "6432")
	t.Setenv("TEST_DATABASE_NAME", "permissions")
	t.Setenv("TEST_DATABASE_USER", "svc")
	t.Setenv("TEST_DATABASE_PASSWORD", "secret")
	t.Setenv("TEST_DATABASE_SSLMODE", "require")

	assert.Equal(t, DatabaseConfig{
		Host:     "db",
		Port:     "6432",
		Name:     "permissions",
		User:     "svc",
		Password: "secret",
		SSLMode:  "require",
	}, DefaultDatabaseConfig())
}
