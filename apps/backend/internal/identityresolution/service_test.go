package identityresolution

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/weibinliao/OpenAD/internal/scanner"
	"gorm.io/gorm"
)

const (
	testUserSID    = "S-1-5-21-1-2-3-1001"
	testNestedSID  = "S-1-5-21-1-2-3-1002"
	testGroupSID   = "S-1-5-21-1-2-3-2001"
	testMissingSID = "S-1-5-21-1-2-3-9999"
)

func TestResolveUsesSnapshotForUsersGroupsAndWellKnownSIDs(t *testing.T) {
	db := newResolverTestDB(t)
	run := seedResolverSnapshot(t, db)
	resolver := NewService(db, Options{RunID: run.ID})

	result, err := resolver.Resolve(context.Background(), []scanner.Permission{
		{Path: `C:\Share`, Trustee: testUserSID, TrusteeSID: testUserSID, Rights: "Read", Type: "Allow"},
		{Path: `C:\Share`, Trustee: testGroupSID, TrusteeSID: testGroupSID, Rights: "Modify", Type: "Allow"},
		{Path: `C:\Share`, Trustee: "S-1-1-0", TrusteeSID: "S-1-1-0", Rights: "Read", Type: "Allow"},
		{Path: `C:\Share`, Trustee: testMissingSID, TrusteeSID: testMissingSID, Rights: "Read", Type: "Allow"},
	})

	require.NoError(t, err)
	assert.Equal(t, run.ID, result.Metadata.DirectorySyncRunID)
	assert.Equal(t, "snapshot", result.Metadata.Mode)
	assert.Equal(t, 3, result.Metadata.ResolvedPrincipalCount)
	assert.Equal(t, 1, result.Metadata.UnresolvedPrincipalCount)

	direct := findResolvedPermission(t, result.Permissions, testUserSID, "")
	assert.Equal(t, "snapshot", direct.ResolutionSource)
	assert.Equal(t, "alice", direct.AccountName)
	assert.Equal(t, "Alice Adams", direct.Trustee)

	groupMember := findResolvedPermission(t, result.Permissions, testNestedSID, "Finance")
	assert.Equal(t, "snapshot", groupMember.ResolutionSource)
	assert.Equal(t, "Finance", groupMember.OriginatingGroup)
	assert.Equal(t, "Helpdesk > Finance", groupMember.GroupInheritanceHierarchy)

	everyone := findResolvedPermission(t, result.Permissions, "S-1-1-0", "")
	assert.Equal(t, "windows", everyone.ResolutionSource)
	assert.Equal(t, "Everyone", everyone.Trustee)

	missing := findResolvedPermission(t, result.Permissions, testMissingSID, "")
	assert.Equal(t, "raw", missing.ResolutionSource)
	assert.Equal(t, "not_in_snapshot", missing.ResolutionReason)
}

func TestResolveSelectsLatestCompletedSnapshotForConnection(t *testing.T) {
	db := newResolverTestDB(t)
	connectionID := uuid.New()
	oldRun := seedResolverRun(t, db, connectionID, time.Now().UTC().Add(-2*time.Hour), "completed")
	newRun := seedResolverRun(t, db, connectionID, time.Now().UTC().Add(-time.Hour), "completed")
	seedResolverUser(t, db, oldRun.ID, models.ADUserRecord{SID: testUserSID, DisplayName: "Old Alice"})
	seedResolverUser(t, db, newRun.ID, models.ADUserRecord{SID: testUserSID, DisplayName: "Current Alice"})
	seedResolverRun(t, db, connectionID, time.Now().UTC(), "failed")

	resolver := NewService(db, Options{ConnectionID: connectionID})
	result, err := resolver.Resolve(context.Background(), []scanner.Permission{{Trustee: testUserSID, TrusteeSID: testUserSID}})

	require.NoError(t, err)
	assert.Equal(t, newRun.ID, result.Metadata.DirectorySyncRunID)
	require.Len(t, result.Permissions, 1)
	assert.Equal(t, "Current Alice", result.Permissions[0].Trustee)
}

func TestResolveSendsOnlySnapshotMissesToLiveLDAP(t *testing.T) {
	db := newResolverTestDB(t)
	run := seedResolverSnapshot(t, db)
	live := &stubLiveExpander{permissions: []scanner.Permission{{
		Path:        `C:\Share`,
		Trustee:     `EXAMPLE\charlie`,
		TrusteeSID:  testMissingSID,
		AccountName: "charlie",
	}}}
	resolver := NewService(db, Options{RunID: run.ID, LiveExpander: live})

	result, err := resolver.Resolve(context.Background(), []scanner.Permission{
		{Path: `C:\Share`, Trustee: testUserSID, TrusteeSID: testUserSID},
		{Path: `C:\Share`, Trustee: testMissingSID, TrusteeSID: testMissingSID},
	})

	require.NoError(t, err)
	require.Len(t, live.received, 1)
	assert.Equal(t, testMissingSID, live.received[0].TrusteeSID)
	livePermission := findResolvedPermission(t, result.Permissions, testMissingSID, "")
	assert.Equal(t, "ldap", livePermission.ResolutionSource)
	assert.Equal(t, "snapshot+ldap", result.Metadata.Mode)
	assert.Equal(t, 2, result.Metadata.ResolvedPrincipalCount)
	assert.Zero(t, result.Metadata.UnresolvedPrincipalCount)
}

func TestResolveKeepsSnapshotResultsWhenLiveLDAPIsUnavailable(t *testing.T) {
	db := newResolverTestDB(t)
	run := seedResolverSnapshot(t, db)
	resolver := NewService(db, Options{RunID: run.ID, LiveUnavailable: true})

	result, err := resolver.Resolve(context.Background(), []scanner.Permission{
		{Trustee: testUserSID, TrusteeSID: testUserSID},
		{Trustee: testMissingSID, TrusteeSID: testMissingSID},
	})

	require.NoError(t, err)
	assert.Equal(t, "snapshot", result.Metadata.Mode)
	assert.Equal(t, 1, result.Metadata.ResolvedPrincipalCount)
	assert.Equal(t, 1, result.Metadata.UnresolvedPrincipalCount)
	assert.NotEmpty(t, result.Metadata.Warning)
	resolved := findResolvedPermission(t, result.Permissions, testUserSID, "")
	assert.Equal(t, "snapshot", resolved.ResolutionSource)
	missing := findResolvedPermission(t, result.Permissions, testMissingSID, "")
	assert.Equal(t, "ldap_unavailable", missing.ResolutionReason)
}

type stubLiveExpander struct {
	received    []scanner.Permission
	permissions []scanner.Permission
	err         error
}

func (stub *stubLiveExpander) Expand(_ context.Context, permissions []scanner.Permission) ([]scanner.Permission, error) {
	stub.received = append([]scanner.Permission(nil), permissions...)
	return append([]scanner.Permission(nil), stub.permissions...), stub.err
}

func newResolverTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.DirectorySyncRun{}, &models.ADUserRecord{}, &models.ADGroupRecord{}, &models.ADMembershipRecord{}))
	return db
}

func seedResolverSnapshot(t *testing.T, db *gorm.DB) models.DirectorySyncRun {
	t.Helper()
	run := seedResolverRun(t, db, uuid.New(), time.Now().UTC(), "completed")
	seedResolverUser(t, db, run.ID, models.ADUserRecord{SID: testUserSID, SAMAccountName: "alice", DisplayName: "Alice Adams", Domain: "CORP"})
	seedResolverUser(t, db, run.ID, models.ADUserRecord{SID: testNestedSID, SAMAccountName: "bob", DisplayName: "Bob Brown", Domain: "CORP"})
	require.NoError(t, db.Create(&models.ADGroupRecord{RunID: run.ID, SID: testGroupSID, Name: "Finance", DN: "CN=Finance,DC=corp,DC=test"}).Error)
	require.NoError(t, db.Create(&[]models.ADMembershipRecord{
		{RunID: run.ID, MemberSID: testUserSID, GroupSID: testGroupSID, Direct: true},
		{RunID: run.ID, MemberSID: testNestedSID, GroupSID: testGroupSID, Direct: false, ViaChain: "Helpdesk > Finance"},
	}).Error)
	return run
}

func seedResolverRun(t *testing.T, db *gorm.DB, connectionID uuid.UUID, startedAt time.Time, status string) models.DirectorySyncRun {
	t.Helper()
	finishedAt := startedAt.Add(time.Minute)
	run := models.DirectorySyncRun{ConnectionID: connectionID, Status: status, StartedAt: startedAt, FinishedAt: &finishedAt}
	require.NoError(t, db.Create(&run).Error)
	return run
}

func seedResolverUser(t *testing.T, db *gorm.DB, runID uuid.UUID, user models.ADUserRecord) {
	t.Helper()
	user.RunID = runID
	require.NoError(t, db.Create(&user).Error)
}

func findResolvedPermission(t *testing.T, permissions []scanner.Permission, sid, originatingGroup string) scanner.Permission {
	t.Helper()
	for _, permission := range permissions {
		if permission.TrusteeSID == sid && permission.OriginatingGroup == originatingGroup {
			return permission
		}
	}
	t.Fatalf("permission sid=%s group=%s not found in %#v", sid, originatingGroup, permissions)
	return scanner.Permission{}
}
