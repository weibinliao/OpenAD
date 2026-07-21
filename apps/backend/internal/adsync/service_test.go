package adsync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/weibinliao/OpenAD/internal/ad"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fakeEnumerator struct {
	users    []ad.SyncUser
	groups   []ad.SyncGroup
	userErr  error
	groupErr error
}

func (fake *fakeEnumerator) EnumerateSyncUsers(_ context.Context, _ int, fn func(ad.SyncUser) error) error {
	if fake.userErr != nil {
		return fake.userErr
	}
	for _, user := range fake.users {
		if err := fn(user); err != nil {
			return err
		}
	}
	return nil
}

func (fake *fakeEnumerator) EnumerateSyncGroups(_ context.Context, _ int, fn func(ad.SyncGroup) error) error {
	if fake.groupErr != nil {
		return fake.groupErr
	}
	for _, group := range fake.groups {
		if err := fn(group); err != nil {
			return err
		}
	}
	return nil
}

func newTestService(t *testing.T) *Service {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	err = db.AutoMigrate(
		&models.DirectorySyncRun{},
		&models.ADUserRecord{},
		&models.ADGroupRecord{},
		&models.ADMembershipRecord{},
	)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return NewService(db)
}

func TestServiceRunCompletes(t *testing.T) {
	service := newTestService(t)

	run, err := service.StartRun(uuid.New())
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if run.Status != "running" {
		t.Fatalf("new run status = %q, want running", run.Status)
	}

	running, err := service.HasRunningRun()
	if err != nil || !running {
		t.Fatalf("HasRunningRun = %v, %v; want true, nil", running, err)
	}

	enumerator := &fakeEnumerator{
		groups: []ad.SyncGroup{
			{DN: "CN=Sales,DC=test", Name: "Sales", SID: "S-G1", Scope: "global", MemberOf: []string{"CN=Staff,DC=test"}},
			{DN: "CN=Staff,DC=test", Name: "Staff", SID: "S-G2", Scope: "universal"},
		},
		users: []ad.SyncUser{
			{
				DN:                "CN=alice,DC=test",
				SAMAccountName:    "alice",
				UserPrincipalName: "alice@test.example",
				DisplayName:       "Alice Example",
				Email:             "alice@test.example",
				FirstName:         "Alice",
				LastName:          "Example",
				Department:        "Finance",
				Division:          "Corporate",
				Domain:            "TEST.EXAMPLE",
				SID:               "S-U1",
				Enabled:           true,
				MemberOf:          []string{"CN=Sales,DC=test"},
			},
			{DN: "CN=bob,DC=test", SAMAccountName: "bob", SID: "S-U2", Enabled: false},
		},
	}

	if err := service.Run(context.Background(), run.ID, enumerator); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, err := service.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != "completed" {
		t.Fatalf("run status = %q (%s), want completed", got.Status, got.ErrorMessage)
	}
	if got.UserCount != 2 || got.GroupCount != 2 {
		t.Fatalf("counts = users %d, groups %d; want 2, 2", got.UserCount, got.GroupCount)
	}
	// alice -> Sales (direct), alice -> Staff (nested), Sales -> Staff (group edge).
	if got.MembershipCount != 3 {
		t.Fatalf("membership count = %d, want 3", got.MembershipCount)
	}
	if got.FinishedAt == nil {
		t.Fatal("finished_at not set")
	}

	var alice models.ADUserRecord
	err = service.db.Where("run_id = ? AND sid = ?", run.ID, "S-U1").First(&alice).Error
	if err != nil {
		t.Fatalf("alice lookup: %v", err)
	}
	if alice.FirstName != "Alice" || alice.LastName != "Example" || alice.Division != "Corporate" || alice.Domain != "TEST.EXAMPLE" {
		t.Fatalf("persisted user fields = first %q last %q division %q domain %q; want Alice, Example, Corporate, TEST.EXAMPLE", alice.FirstName, alice.LastName, alice.Division, alice.Domain)
	}

	var membershipRows int64
	if err := service.db.Model(&models.ADMembershipRecord{}).Where("run_id = ?", run.ID).Count(&membershipRows).Error; err != nil {
		t.Fatalf("count memberships: %v", err)
	}
	if membershipRows != 3 {
		t.Fatalf("persisted membership rows = %d, want 3", membershipRows)
	}

	var nested models.ADMembershipRecord
	err = service.db.Where("run_id = ? AND member_sid = ? AND group_sid = ?", run.ID, "S-U1", "S-G2").First(&nested).Error
	if err != nil {
		t.Fatalf("nested edge lookup: %v", err)
	}
	if nested.Direct || nested.ViaChain != "Sales > Staff" {
		t.Fatalf("nested edge = direct %v via %q, want indirect via 'Sales > Staff'", nested.Direct, nested.ViaChain)
	}

	running, err = service.HasRunningRun()
	if err != nil || running {
		t.Fatalf("HasRunningRun after completion = %v, %v; want false, nil", running, err)
	}
}

func TestServiceRunMarksFailed(t *testing.T) {
	service := newTestService(t)

	run, err := service.StartRun(uuid.New())
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	enumerator := &fakeEnumerator{groupErr: errors.New("ldap unavailable")}
	if err := service.Run(context.Background(), run.ID, enumerator); err == nil {
		t.Fatal("Run should return the enumeration error")
	}

	got, err := service.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != "failed" {
		t.Fatalf("run status = %q, want failed", got.Status)
	}
	if got.ErrorMessage == "" || got.FinishedAt == nil {
		t.Fatalf("failed run must have error message and finished_at; got %q, %v", got.ErrorMessage, got.FinishedAt)
	}
}

func TestServiceListRuns(t *testing.T) {
	service := newTestService(t)

	first, err := service.StartRun(uuid.New())
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := service.finishRun(first.ID, "completed", "", nil); err != nil {
		t.Fatalf("finishRun: %v", err)
	}
	// Push the first run into the past so ordering is deterministic even if
	// both StartRun calls land on the same timestamp granularity.
	earlier := time.Now().UTC().Add(-time.Hour)
	if err := service.db.Model(&models.DirectorySyncRun{}).Where("id = ?", first.ID).Update("started_at", earlier).Error; err != nil {
		t.Fatalf("backdate first run: %v", err)
	}
	second, err := service.StartRun(uuid.New())
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	runs, total, err := service.ListRuns(1, 20)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if total != 2 || len(runs) != 2 {
		t.Fatalf("ListRuns total = %d, len = %d; want 2, 2", total, len(runs))
	}
	if runs[0].ID != second.ID {
		t.Fatalf("runs not ordered by started_at desc; first = %s, want %s", runs[0].ID, second.ID)
	}

	if _, err := service.GetRun(uuid.New()); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("GetRun unknown id error = %v, want ErrRunNotFound", err)
	}
}
