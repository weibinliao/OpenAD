package testutil

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultTestDatabaseHost     = "127.0.0.1"
	defaultTestDatabasePort     = "5433"
	defaultTestDatabaseName     = "folder_security_test"
	defaultTestDatabaseUser     = "test"
	defaultTestDatabasePassword = "test"
	defaultTestDatabaseSSLMode  = "disable"
)

type DatabaseConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

func DefaultDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Host:     envOrDefault("TEST_DATABASE_HOST", defaultTestDatabaseHost),
		Port:     envOrDefault("TEST_DATABASE_PORT", defaultTestDatabasePort),
		Name:     envOrDefault("TEST_DATABASE_NAME", defaultTestDatabaseName),
		User:     envOrDefault("TEST_DATABASE_USER", defaultTestDatabaseUser),
		Password: envOrDefault("TEST_DATABASE_PASSWORD", defaultTestDatabasePassword),
		SSLMode:  envOrDefault("TEST_DATABASE_SSLMODE", defaultTestDatabaseSSLMode),
	}
}

func ResolveTestDatabaseURL() string {
	if databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")); databaseURL != "" {
		return databaseURL
	}

	return DefaultDatabaseConfig().URL()
}

func (config DatabaseConfig) URL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		strings.TrimSpace(config.User),
		strings.TrimSpace(config.Password),
		strings.TrimSpace(config.Host),
		strings.TrimSpace(config.Port),
		strings.TrimSpace(config.Name),
		strings.TrimSpace(config.SSLMode),
	)
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return fallback
}
