package connections

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/secrets"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	secrets.ResetForTest()
	t.Cleanup(secrets.ResetForTest)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ADConnectionProfile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Isolate per test: drop rows left by shared cache.
	db.Exec("DELETE FROM ad_connection_profiles")

	return NewService(db, t.TempDir())
}

func TestCreateEncryptsPassword(t *testing.T) {
	service := newTestService(t)

	profile, err := service.Create(ProfileInput{
		Name:     "example-dc",
		Server:   "ldap://dc01.example.com",
		BaseDN:   "DC=example,DC=com",
		BindUser: "EXAMPLE\\svc-reader",
		Password: "Secr3t!",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if profile.EncryptedPassword == "" || strings.Contains(profile.EncryptedPassword, "Secr3t!") {
		t.Fatal("password not encrypted at rest")
	}

	resolved, err := service.Resolve(profile.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Password != "Secr3t!" {
		t.Fatalf("resolved password mismatch: %q", resolved.Password)
	}
}

func TestCreateNormalizesDNSDomainBackslashAccount(t *testing.T) {
	service := newTestService(t)

	profile, err := service.Create(ProfileInput{
		Name:     "example-dc",
		Server:   "ldap://dc01.example.com",
		BaseDN:   "DC=example,DC=com",
		BindUser: "example.com\\alice",
		Password: "Secr3t!",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if profile.BindUser != "alice@example.com" {
		t.Fatalf("expected normalized UPN, got %q", profile.BindUser)
	}
}

func TestUpdateKeepsPasswordWhenEmpty(t *testing.T) {
	service := newTestService(t)

	profile, err := service.Create(ProfileInput{
		Name: "example", Server: "dc01", BaseDN: "DC=example", BindUser: "u", Password: "keepme",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := service.Update(profile.ID, ProfileInput{
		Name: "example-renamed", Server: "dc02", BaseDN: "DC=example", BindUser: "u2", Password: "",
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	resolved, err := service.Resolve(profile.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Password != "keepme" {
		t.Fatalf("password should be preserved, got %q", resolved.Password)
	}
	if resolved.Server != "dc02" {
		t.Fatalf("server not updated: %q", resolved.Server)
	}
}

func TestDefaultSelection(t *testing.T) {
	service := newTestService(t)

	first, err := service.Create(ProfileInput{Name: "a", Server: "s", BaseDN: "d", BindUser: "u", Password: "p"})
	if err != nil {
		t.Fatalf("Create a: %v", err)
	}

	// Single profile acts as default even without the flag.
	def, err := service.Default()
	if err != nil {
		t.Fatalf("Default with one profile: %v", err)
	}
	if def.ID != first.ID {
		t.Fatal("single profile should be default")
	}

	second, err := service.Create(ProfileInput{Name: "b", Server: "s", BaseDN: "d", BindUser: "u", Password: "p", IsDefault: true})
	if err != nil {
		t.Fatalf("Create b: %v", err)
	}

	def, err = service.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	if def.ID != second.ID {
		t.Fatal("flagged profile should be default")
	}

	// Setting a new default clears the old flag.
	if _, err := service.Update(first.ID, ProfileInput{Name: "a", Server: "s", BaseDN: "d", BindUser: "u", IsDefault: true}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	var count int64
	service.db.Model(&models.ADConnectionProfile{}).Where("is_default = ?", true).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly one default, got %d", count)
	}
}

func TestDeleteMissingReturnsNotFound(t *testing.T) {
	service := newTestService(t)
	profile, err := service.Create(ProfileInput{Name: "x", Server: "s", BaseDN: "d", BindUser: "u", Password: "p"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := service.Delete(profile.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := service.Delete(profile.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
