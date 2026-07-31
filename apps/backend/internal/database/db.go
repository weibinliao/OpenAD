package database

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/weibinliao/OpenAD/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

var StoreDescription string

const sqliteBusyTimeoutMilliseconds = 5000

func InitWithDSN(dsn string) error {
	if strings.TrimSpace(dsn) == "" {
		return errors.New("database dsn is required")
	}

	if DB != nil {
		return nil
	}

	dialector, description, err := dialectorForDSN(dsn)
	if err != nil {
		return err
	}

	DB, err = gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return err
	}
	if dialector.Name() == "sqlite" {
		sqlDB, poolErr := DB.DB()
		if poolErr != nil {
			return poolErr
		}
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	}
	StoreDescription = description

	err = DB.AutoMigrate(
		&models.ScanSession{},
		&models.Permission{},
		&models.PermissionChange{},
		&models.RiskFinding{},
		&models.ADConnectionProfile{},
		&models.DirectorySyncRun{},
		&models.ADUserRecord{},
		&models.ADGroupRecord{},
		&models.ADMembershipRecord{},
	)
	if err != nil {
		return err
	}

	log.Printf("Database connected and migrated: %s", StoreDescription)

	return nil
}

func Init() error {
	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		dsn = defaultSQLiteDSN()
	}

	return InitWithDSN(dsn)
}

func InitFromEnv() error {
	return Init()
}

func Ready() bool {
	return DB != nil
}

func dialectorForDSN(dsn string) (gorm.Dialector, string, error) {
	trimmed := strings.TrimSpace(dsn)
	lower := strings.ToLower(trimmed)

	if strings.HasPrefix(lower, "sqlite://") || strings.HasPrefix(lower, "file:") {
		path, err := sqlitePathFromDSN(trimmed)
		if err != nil {
			return nil, "", err
		}
		if err := ensureSQLiteDirectory(path); err != nil {
			return nil, "", err
		}

		return sqlite.Open(sqliteDSNWithConcurrencyPragmas(path)), "sqlite:" + path, nil
	}

	return postgres.Open(trimmed), "postgres", nil
}

func sqliteDSNWithConcurrencyPragmas(dsn string) string {
	// glebarez/sqlite forwards its DSN to modernc, whose repeated _pragma
	// parameter is applied whenever the database/sql pool opens a connection.
	parameters := url.Values{}
	parameters.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", sqliteBusyTimeoutMilliseconds))
	parameters.Add("_pragma", "journal_mode(WAL)")

	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameters.Encode()
}

func defaultSQLiteDSN() string {
	return "sqlite://" + defaultSQLitePath()
}

func defaultSQLitePath() string {
	return filepath.Join(DataDir(), "permission-protector.db")
}

// DataDir resolves the application data directory using the same precedence
// as the default SQLite location. Other subsystems (e.g. the credential key
// file) should store their state here.
func DataDir() string {
	dataDir := strings.TrimSpace(os.Getenv("PERMISSION_PROTECTOR_DATA_DIR"))
	if dataDir == "" {
		if configDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(configDir) != "" {
			dataDir = filepath.Join(configDir, "PermissionProtector")
		}
	}
	if dataDir == "" {
		if homeDir, err := os.UserHomeDir(); err == nil && strings.TrimSpace(homeDir) != "" {
			dataDir = filepath.Join(homeDir, ".permission-protector")
		}
	}
	if dataDir == "" {
		dataDir = filepath.Join(os.TempDir(), "PermissionProtector")
	}

	return dataDir
}

func sqlitePathFromDSN(dsn string) (string, error) {
	trimmed := strings.TrimSpace(dsn)
	if strings.HasPrefix(strings.ToLower(trimmed), "sqlite://") {
		path := trimmed[len("sqlite://"):]
		if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
			path = path[1:]
		}
		path = strings.TrimSpace(path)
		if path == "" {
			return "", errors.New("sqlite database path is required")
		}
		return path, nil
	}

	return trimmed, nil
}

func ensureSQLiteDirectory(path string) error {
	if strings.HasPrefix(strings.ToLower(path), "file:") {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || strings.TrimSpace(dir) == "" {
		return nil
	}

	return os.MkdirAll(dir, 0o700)
}
