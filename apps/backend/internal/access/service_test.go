package access

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/weibinliao/OpenAD/internal/models"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	err = db.AutoMigrate(
		&models.ScanSession{},
		&models.Permission{},
		&models.DirectorySyncRun{},
		&models.ADUserRecord{},
		&models.ADGroupRecord{},
		&models.ADMembershipRecord{},
	)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// fixture builds one completed sync run (alice direct in Sales, nested in
// Staff via Sales; bob in nothing) and one completed scan session with a user
// ACE, group ACEs and an Everyone ACE.
type fixture struct {
	runID     uuid.UUID
	sessionID uuid.UUID
}

const (
	aliceSID    = "S-1-5-21-1-1-1-1001"
	bobSID      = "S-1-5-21-1-1-1-1002"
	salesSID    = "S-1-5-21-1-1-1-2001"
	staffSID    = "S-1-5-21-1-1-1-2002"
	everyoneSID = "S-1-1-0"
)

func seedFixture(t *testing.T, db *gorm.DB) fixture {
	t.Helper()

	now := time.Now().UTC()
	finished := now.Add(time.Minute)

	run := models.DirectorySyncRun{
		ConnectionID: uuid.New(),
		Status:       "completed",
		StartedAt:    now,
		FinishedAt:   &finished,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("seed run: %v", err)
	}

	users := []models.ADUserRecord{
		{RunID: run.ID, SID: aliceSID, SAMAccountName: "alice", UPN: "alice@corp.test", DisplayName: "Alice Adams", Enabled: true},
		{RunID: run.ID, SID: bobSID, SAMAccountName: "bob", UPN: "bob@corp.test", DisplayName: "Bob Brown", Enabled: true},
	}
	groups := []models.ADGroupRecord{
		{RunID: run.ID, SID: salesSID, Name: "Sales-Team"},
		{RunID: run.ID, SID: staffSID, Name: "All-Staff"},
	}
	memberships := []models.ADMembershipRecord{
		{RunID: run.ID, MemberSID: aliceSID, GroupSID: salesSID, Direct: true},
		{RunID: run.ID, MemberSID: aliceSID, GroupSID: staffSID, Direct: false, ViaChain: "Sales-Team > All-Staff"},
		{RunID: run.ID, MemberSID: salesSID, GroupSID: staffSID, Direct: true},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := db.Create(&groups).Error; err != nil {
		t.Fatalf("seed groups: %v", err)
	}
	if err := db.Create(&memberships).Error; err != nil {
		t.Fatalf("seed memberships: %v", err)
	}

	session := models.ScanSession{
		RootPath:   `D:\Share`,
		Status:     "completed",
		StartedAt:  now,
		FinishedAt: &finished,
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("seed session: %v", err)
	}

	permissions := []models.Permission{
		{ScanSessionID: session.ID, Path: `D:\Share\HR`, Trustee: `CORP\alice`, TrusteeSID: aliceSID, Rights: "FullControl", Type: "allow", RiskLevel: "high"},
		{ScanSessionID: session.ID, Path: `D:\Share\Sales`, Trustee: `CORP\Sales-Team`, TrusteeSID: salesSID, Rights: "Modify", Type: "allow", RiskLevel: "medium"},
		{ScanSessionID: session.ID, Path: `D:\Share\Common`, Trustee: `CORP\All-Staff`, TrusteeSID: staffSID, Rights: "ReadAndExecute", Type: "allow", Inherited: true},
		{ScanSessionID: session.ID, Path: `D:\Share\Public`, Trustee: "Everyone", TrusteeSID: everyoneSID, Rights: "FullControl", Type: "allow", RiskLevel: "critical"},
	}
	if err := db.Create(&permissions).Error; err != nil {
		t.Fatalf("seed permissions: %v", err)
	}

	return fixture{runID: run.ID, sessionID: session.ID}
}

func TestTrusteeSIDColumnName(t *testing.T) {
	service := NewService(newTestDB(t))
	// Verified empirically: GORM maps Permission.TrusteeSID to trustee_s_id,
	// NOT trustee_sid ("SID" is not a GORM common initialism).
	if service.trusteeSIDColumn != "trustee_s_id" {
		t.Fatalf("trustee SID column = %q, want trustee_s_id", service.trusteeSIDColumn)
	}
}

func TestByUserDirectAndViaGroup(t *testing.T) {
	db := newTestDB(t)
	seed := seedFixture(t, db)
	service := NewService(db)

	for _, principal := range []string{aliceSID, "alice", "ALICE@corp.test"} {
		result, err := service.ByUser(ByUserInput{Principal: principal})
		if err != nil {
			t.Fatalf("ByUser(%q): %v", principal, err)
		}

		if result.SyncRunID != seed.runID {
			t.Fatalf("sync run = %s, want %s", result.SyncRunID, seed.runID)
		}
		if result.User.SID != aliceSID || result.User.SAMAccountName != "alice" {
			t.Fatalf("resolved user = %+v", result.User)
		}
		if result.GroupCount != 2 {
			t.Fatalf("group count = %d, want 2", result.GroupCount)
		}
		if len(result.Entries) != 3 {
			t.Fatalf("entries = %d, want 3 (direct + Sales + Staff)", len(result.Entries))
		}
		if result.Counts.Direct != 1 || result.Counts.ViaGroup != 2 || result.Counts.Allow != 3 {
			t.Fatalf("counts = %+v", result.Counts)
		}

		byPath := make(map[string]AccessEntry, len(result.Entries))
		for _, entry := range result.Entries {
			byPath[entry.Path] = entry
		}

		direct := byPath[`D:\Share\HR`]
		if direct.Why.Kind != "direct" || direct.Why.Description != "direct" || direct.Rights != "FullControl" {
			t.Fatalf("direct entry = %+v", direct)
		}

		viaSales := byPath[`D:\Share\Sales`]
		if viaSales.Why.Kind != "group" || viaSales.Why.GroupName != "Sales-Team" || viaSales.Why.GroupSID != salesSID {
			t.Fatalf("via-Sales entry = %+v", viaSales)
		}
		if viaSales.Why.ViaChain != "" {
			t.Fatalf("direct group membership should have empty via chain, got %q", viaSales.Why.ViaChain)
		}
		if viaSales.Why.Description != "via group Sales-Team" {
			t.Fatalf("via-Sales description = %q", viaSales.Why.Description)
		}

		viaStaff := byPath[`D:\Share\Common`]
		if viaStaff.Why.Kind != "group" || viaStaff.Why.GroupName != "All-Staff" {
			t.Fatalf("via-Staff entry = %+v", viaStaff)
		}
		if viaStaff.Why.ViaChain != "Sales-Team > All-Staff" {
			t.Fatalf("nested via chain = %q, want 'Sales-Team > All-Staff'", viaStaff.Why.ViaChain)
		}
		if viaStaff.Why.Description != "via group All-Staff (Sales-Team > All-Staff)" {
			t.Fatalf("via-Staff description = %q", viaStaff.Why.Description)
		}
		if !viaStaff.Inherited {
			t.Fatal("via-Staff entry should carry Inherited=true")
		}

		// The Everyone ACE must NOT appear: it is not tied to alice's SIDs.
		if _, found := byPath[`D:\Share\Public`]; found {
			t.Fatal("Everyone ACE must not be attributed to a specific user in by-user results")
		}

		if len(result.ByRootPath) != 1 || result.ByRootPath[0].RootPath != `D:\Share` || len(result.ByRootPath[0].Entries) != 3 {
			t.Fatalf("by_root_path grouping = %+v", result.ByRootPath)
		}
		if len(result.Sessions) != 1 || result.Sessions[0].ID != seed.sessionID {
			t.Fatalf("sessions consulted = %+v", result.Sessions)
		}
	}
}

func TestByUserNoGroupAccess(t *testing.T) {
	db := newTestDB(t)
	seedFixture(t, db)
	service := NewService(db)

	result, err := service.ByUser(ByUserInput{Principal: "bob"})
	if err != nil {
		t.Fatalf("ByUser: %v", err)
	}
	if result.GroupCount != 0 || len(result.Entries) != 0 || result.Counts.Total != 0 {
		t.Fatalf("bob should have no entries, got %+v", result)
	}
}

func TestByUserExplicitSessionAndRunSelection(t *testing.T) {
	db := newTestDB(t)
	seed := seedFixture(t, db)
	service := NewService(db)

	result, err := service.ByUser(ByUserInput{
		Principal:  "alice",
		SyncRunID:  seed.runID,
		SessionIDs: []uuid.UUID{seed.sessionID},
	})
	if err != nil {
		t.Fatalf("ByUser explicit: %v", err)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(result.Entries))
	}

	if _, err := service.ByUser(ByUserInput{Principal: "alice", SyncRunID: uuid.New()}); !errors.Is(err, ErrSyncRunNotFound) {
		t.Fatalf("unknown sync run error = %v, want ErrSyncRunNotFound", err)
	}
	if _, err := service.ByUser(ByUserInput{Principal: "alice", SessionIDs: []uuid.UUID{uuid.New()}}); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("unknown session error = %v, want ErrSessionNotFound", err)
	}
}

func TestByUserUsesLatestCompletedSessionPerRoot(t *testing.T) {
	db := newTestDB(t)
	seed := seedFixture(t, db)
	service := NewService(db)

	// A newer completed session for the same root supersedes the fixture one;
	// a running session must be ignored entirely.
	newer := models.ScanSession{RootPath: `D:\Share`, Status: "completed", StartedAt: time.Now().UTC().Add(time.Hour)}
	if err := db.Create(&newer).Error; err != nil {
		t.Fatalf("seed newer session: %v", err)
	}
	running := models.ScanSession{RootPath: `E:\Other`, Status: "running", StartedAt: time.Now().UTC()}
	if err := db.Create(&running).Error; err != nil {
		t.Fatalf("seed running session: %v", err)
	}
	if err := db.Create(&models.Permission{
		ScanSessionID: newer.ID, Path: `D:\Share\New`, Trustee: `CORP\alice`,
		TrusteeSID: aliceSID, Rights: "Read", Type: "allow",
	}).Error; err != nil {
		t.Fatalf("seed newer permission: %v", err)
	}

	result, err := service.ByUser(ByUserInput{Principal: "alice"})
	if err != nil {
		t.Fatalf("ByUser: %v", err)
	}
	if len(result.Sessions) != 1 || result.Sessions[0].ID != newer.ID {
		t.Fatalf("sessions = %+v, want only newest completed for D:\\Share (%s), fixture was %s", result.Sessions, newer.ID, seed.sessionID)
	}
	if len(result.Entries) != 1 || result.Entries[0].Path != `D:\Share\New` {
		t.Fatalf("entries = %+v", result.Entries)
	}
}

func TestByUserErrors(t *testing.T) {
	db := newTestDB(t)
	service := NewService(db)

	if _, err := service.ByUser(ByUserInput{Principal: "  "}); !errors.Is(err, ErrPrincipalRequired) {
		t.Fatalf("blank principal error = %v", err)
	}
	// No sync run at all.
	if _, err := service.ByUser(ByUserInput{Principal: "alice"}); !errors.Is(err, ErrNoCompletedSyncRun) {
		t.Fatalf("no sync run error = %v, want ErrNoCompletedSyncRun", err)
	}

	seedFixture(t, db)
	if _, err := service.ByUser(ByUserInput{Principal: "nobody-here"}); !errors.Is(err, ErrPrincipalNotFound) {
		t.Fatalf("unknown principal error = %v, want ErrPrincipalNotFound", err)
	}

	// Sync run present but no completed scan sessions.
	if err := db.Where("1 = 1").Delete(&models.ScanSession{}).Error; err != nil {
		t.Fatalf("delete sessions: %v", err)
	}
	if _, err := service.ByUser(ByUserInput{Principal: "alice"}); !errors.Is(err, ErrNoScanSessions) {
		t.Fatalf("no sessions error = %v, want ErrNoScanSessions", err)
	}
}

func TestByResourceExpandsGroupsAndKeepsUnresolved(t *testing.T) {
	db := newTestDB(t)
	seed := seedFixture(t, db)
	service := NewService(db)

	result, err := service.ByResource(ByResourceInput{PathPrefix: `D:\Share`})
	if err != nil {
		t.Fatalf("ByResource: %v", err)
	}

	if result.Session.ID != seed.sessionID || result.SyncRunID != seed.runID {
		t.Fatalf("session/run = %s / %s", result.Session.ID, result.SyncRunID)
	}
	if len(result.ACEs) != 4 {
		t.Fatalf("raw ACEs = %d, want 4", len(result.ACEs))
	}

	type key struct{ sid, source, groupSID string }
	principals := make(map[key]ResourcePrincipal, len(result.Principals))
	for _, principal := range result.Principals {
		principals[key{principal.SID, principal.Source, principal.GroupSID}] = principal
	}

	// Alice directly (her own ACE).
	direct, found := principals[key{aliceSID, "user", ""}]
	if !found || direct.Name != "Alice Adams" {
		t.Fatalf("missing direct alice principal: %+v", result.Principals)
	}
	if len(direct.Rights) != 1 || direct.Rights[0] != "FullControl" || direct.RiskLevel != "high" {
		t.Fatalf("direct alice = %+v", direct)
	}

	// Alice via Sales-Team (group expansion, direct membership).
	viaSales, found := principals[key{aliceSID, "group-member", salesSID}]
	if !found || viaSales.GroupName != "Sales-Team" || viaSales.ViaChain != "" {
		t.Fatalf("missing/wrong alice-via-Sales principal: %+v", viaSales)
	}

	// Alice via All-Staff with the nested via chain preserved.
	viaStaff, found := principals[key{aliceSID, "group-member", staffSID}]
	if !found || viaStaff.GroupName != "All-Staff" {
		t.Fatalf("missing alice-via-Staff principal: %+v", result.Principals)
	}
	if viaStaff.ViaChain != "Sales-Team > All-Staff" {
		t.Fatalf("nested via chain = %q", viaStaff.ViaChain)
	}

	// Everyone must be present, labeled, and flagged unresolved.
	everyone, found := principals[key{everyoneSID, "unresolved", ""}]
	if !found {
		t.Fatalf("unresolved Everyone trustee was dropped: %+v", result.Principals)
	}
	if everyone.Name != "Everyone" || everyone.RiskLevel != "critical" {
		t.Fatalf("Everyone principal = %+v", everyone)
	}

	if result.Counts.Users != 1 || result.Counts.Unresolved != 1 || result.Counts.ViaGroups < 2 {
		t.Fatalf("counts = %+v", result.Counts)
	}
}

func TestByResourceUsesOriginatingGroupAsParent(t *testing.T) {
	db := newTestDB(t)
	seed := seedFixture(t, db)
	service := NewService(db)

	if err := db.Where("scan_session_id = ?", seed.sessionID).Delete(&models.Permission{}).Error; err != nil {
		t.Fatalf("clear fixture permissions: %v", err)
	}
	if err := db.Create(&models.Permission{
		ScanSessionID:             seed.sessionID,
		Path:                      `D:\Share\Sales`,
		Trustee:                   `CORP\alice`,
		TrusteeSID:                aliceSID,
		Rights:                    "Modify",
		Type:                      "allow",
		RiskLevel:                 "medium",
		OriginatingGroup:          "Sales-Team",
		GroupInheritanceHierarchy: "Sales-Team",
	}).Error; err != nil {
		t.Fatalf("seed expanded group permission: %v", err)
	}

	result, err := service.ByResource(ByResourceInput{PathPrefix: `D:\Share`})
	if err != nil {
		t.Fatalf("ByResource: %v", err)
	}
	if len(result.Principals) != 2 {
		t.Fatalf("principals = %+v, want group parent and one member", result.Principals)
	}
	group, member := result.Principals[0], result.Principals[1]
	if group.Source != "group" || group.SID != salesSID || group.Name != "Sales-Team" {
		t.Fatalf("group parent = %+v", group)
	}
	if member.Source != "group-member" || member.SID != aliceSID || member.GroupSID != salesSID {
		t.Fatalf("group member = %+v", member)
	}
	if result.Counts.Users != 0 || result.Counts.ViaGroups != 1 {
		t.Fatalf("counts = %+v", result.Counts)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if !strings.Contains(string(payload), `"member_count":1`) || !strings.Contains(string(payload), `"groups":1`) {
		t.Fatalf("result is missing group count metadata: %s", payload)
	}
}

func TestByResourceKeepsGroupWithoutSnapshotMembers(t *testing.T) {
	db := newTestDB(t)
	seed := seedFixture(t, db)
	service := NewService(db)

	const emptyGroupSID = "S-1-5-21-1-1-1-2513"
	if err := db.Create(&models.ADGroupRecord{
		RunID: seed.runID,
		SID:   emptyGroupSID,
		Name:  "Domain Users",
	}).Error; err != nil {
		t.Fatalf("seed empty group: %v", err)
	}
	if err := db.Where("scan_session_id = ?", seed.sessionID).Delete(&models.Permission{}).Error; err != nil {
		t.Fatalf("clear fixture permissions: %v", err)
	}
	if err := db.Create(&models.Permission{
		ScanSessionID: seed.sessionID,
		Path:          `D:\Share`,
		Trustee:       `CORP\Domain Users`,
		TrusteeSID:    emptyGroupSID,
		Rights:        "ReadAndExecute",
		Type:          "allow",
	}).Error; err != nil {
		t.Fatalf("seed empty group permission: %v", err)
	}

	result, err := service.ByResource(ByResourceInput{PathPrefix: `D:\Share`})
	if err != nil {
		t.Fatalf("ByResource: %v", err)
	}
	if len(result.Principals) != 1 {
		t.Fatalf("principals = %+v, want the empty group parent", result.Principals)
	}
	group := result.Principals[0]
	if group.Source != "group" || group.SID != emptyGroupSID || group.Name != "Domain Users" {
		t.Fatalf("empty group parent = %+v", group)
	}
	if result.Counts.Principals != 1 {
		t.Fatalf("counts = %+v", result.Counts)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if !strings.Contains(string(payload), `"groups":1`) {
		t.Fatalf("result is missing group count metadata: %s", payload)
	}
}

func TestByResourcePathPrefixFiltering(t *testing.T) {
	db := newTestDB(t)
	seedFixture(t, db)
	service := NewService(db)

	result, err := service.ByResource(ByResourceInput{PathPrefix: `D:\Share\Public`})
	if err != nil {
		t.Fatalf("ByResource: %v", err)
	}
	if len(result.ACEs) != 1 || result.ACEs[0].TrusteeSID != everyoneSID {
		t.Fatalf("ACEs under D:\\Share\\Public = %+v", result.ACEs)
	}
	if len(result.Principals) != 1 || result.Principals[0].Source != "unresolved" {
		t.Fatalf("principals = %+v", result.Principals)
	}
}

func TestByResourceErrors(t *testing.T) {
	db := newTestDB(t)
	service := NewService(db)

	if _, err := service.ByResource(ByResourceInput{}); !errors.Is(err, ErrPathRequired) {
		t.Fatalf("blank path error = %v", err)
	}
	if _, err := service.ByResource(ByResourceInput{PathPrefix: `D:\Share`}); !errors.Is(err, ErrNoCompletedSyncRun) {
		t.Fatalf("no sync run error = %v, want ErrNoCompletedSyncRun", err)
	}

	seedFixture(t, db)
	if _, err := service.ByResource(ByResourceInput{PathPrefix: `Z:\Elsewhere`}); !errors.Is(err, ErrNoMatchingSession) {
		t.Fatalf("uncovered path error = %v, want ErrNoMatchingSession", err)
	}
	if _, err := service.ByResource(ByResourceInput{PathPrefix: `D:\Share`, SessionID: uuid.New()}); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("unknown session error = %v, want ErrSessionNotFound", err)
	}

	if err := db.Where("1 = 1").Delete(&models.ScanSession{}).Error; err != nil {
		t.Fatalf("delete sessions: %v", err)
	}
	if _, err := service.ByResource(ByResourceInput{PathPrefix: `D:\Share`}); !errors.Is(err, ErrNoScanSessions) {
		t.Fatalf("no sessions error = %v, want ErrNoScanSessions", err)
	}
}

func TestPathsRelated(t *testing.T) {
	cases := []struct {
		root, query string
		want        bool
	}{
		{`D:\Share`, `D:\Share`, true},
		{`D:\Share`, `d:\share\hr`, true},
		{`D:\Share\HR`, `D:\Share`, true}, // deeper scan still answers a share-root question
		{`D:\Share`, `D:\ShareOther`, false},
		{`D:\Share\`, `D:/Share/HR`, true},
	}
	for _, testCase := range cases {
		if got := pathsRelated(testCase.root, testCase.query); got != testCase.want {
			t.Errorf("pathsRelated(%q, %q) = %v, want %v", testCase.root, testCase.query, got, testCase.want)
		}
	}
}
