package adsync

import (
	"testing"

	"github.com/weibinliao/OpenAD/internal/ad"
	"github.com/weibinliao/OpenAD/internal/models"
)

func group(name, dn, sid string, memberOf ...string) ad.SyncGroup {
	return ad.SyncGroup{DN: dn, Name: name, SID: sid, MemberOf: memberOf}
}

func user(sid string, memberOf ...string) ad.SyncUser {
	return ad.SyncUser{DN: "CN=" + sid + ",DC=test", SID: sid, MemberOf: memberOf}
}

type edgeKey struct {
	member string
	group  string
}

func indexEdges(t *testing.T, records []models.ADMembershipRecord) map[edgeKey]models.ADMembershipRecord {
	t.Helper()
	indexed := make(map[edgeKey]models.ADMembershipRecord, len(records))
	for _, record := range records {
		key := edgeKey{member: record.MemberSID, group: record.GroupSID}
		if _, duplicate := indexed[key]; duplicate {
			t.Fatalf("duplicate membership edge %s -> %s", record.MemberSID, record.GroupSID)
		}
		indexed[key] = record
	}
	return indexed
}

func requireEdge(t *testing.T, edges map[edgeKey]models.ADMembershipRecord, member, group string, direct bool, viaChain string) {
	t.Helper()
	record, ok := edges[edgeKey{member: member, group: group}]
	if !ok {
		t.Fatalf("missing membership edge %s -> %s", member, group)
	}
	if record.Direct != direct {
		t.Fatalf("edge %s -> %s: direct = %v, want %v", member, group, record.Direct, direct)
	}
	if record.ViaChain != viaChain {
		t.Fatalf("edge %s -> %s: via_chain = %q, want %q", member, group, record.ViaChain, viaChain)
	}
}

func TestFlattenMembershipsNestedChain(t *testing.T) {
	groups := []ad.SyncGroup{
		group("Sales-Team", "CN=Sales-Team,DC=test", "S-1-5-21-1-1-1-100", "CN=All-Staff,DC=test"),
		group("All-Staff", "CN=All-Staff,DC=test", "S-1-5-21-1-1-1-200", "CN=Everyone-Group,DC=test"),
		group("Everyone-Group", "CN=Everyone-Group,DC=test", "S-1-5-21-1-1-1-300"),
	}
	users := []ad.SyncUser{user("S-1-5-21-1-1-1-1000", "CN=Sales-Team,DC=test")}

	records, err := FlattenMemberships(users, groups)
	if err != nil {
		t.Fatalf("FlattenMemberships returned error: %v", err)
	}

	edges := indexEdges(t, records)
	// User closure.
	requireEdge(t, edges, "S-1-5-21-1-1-1-1000", "S-1-5-21-1-1-1-100", true, "Sales-Team")
	requireEdge(t, edges, "S-1-5-21-1-1-1-1000", "S-1-5-21-1-1-1-200", false, "Sales-Team > All-Staff")
	requireEdge(t, edges, "S-1-5-21-1-1-1-1000", "S-1-5-21-1-1-1-300", false, "Sales-Team > All-Staff > Everyone-Group")
	// Group closure.
	requireEdge(t, edges, "S-1-5-21-1-1-1-100", "S-1-5-21-1-1-1-200", true, "All-Staff")
	requireEdge(t, edges, "S-1-5-21-1-1-1-100", "S-1-5-21-1-1-1-300", false, "All-Staff > Everyone-Group")
	requireEdge(t, edges, "S-1-5-21-1-1-1-200", "S-1-5-21-1-1-1-300", true, "Everyone-Group")

	if len(records) != 6 {
		t.Fatalf("expected 6 edges, got %d", len(records))
	}
}

func TestFlattenMembershipsDiamond(t *testing.T) {
	// A -> B -> D and A -> C -> D: user in A must reach D exactly once.
	groups := []ad.SyncGroup{
		group("A", "CN=A,DC=test", "S-A", "CN=B,DC=test", "CN=C,DC=test"),
		group("B", "CN=B,DC=test", "S-B", "CN=D,DC=test"),
		group("C", "CN=C,DC=test", "S-C", "CN=D,DC=test"),
		group("D", "CN=D,DC=test", "S-D"),
	}
	users := []ad.SyncUser{user("S-U", "CN=A,DC=test")}

	records, err := FlattenMemberships(users, groups)
	if err != nil {
		t.Fatalf("FlattenMemberships returned error: %v", err)
	}

	edges := indexEdges(t, records) // indexEdges fails the test on duplicates
	requireEdge(t, edges, "S-U", "S-A", true, "A")
	requireEdge(t, edges, "S-U", "S-B", false, "A > B")
	requireEdge(t, edges, "S-U", "S-C", false, "A > C")
	requireEdge(t, edges, "S-U", "S-D", false, "A > B > D") // BFS: shortest chain, first parent wins

	userEdgeCount := 0
	for _, record := range records {
		if record.MemberSID == "S-U" {
			userEdgeCount++
		}
	}
	if userEdgeCount != 4 {
		t.Fatalf("expected 4 user edges in diamond, got %d", userEdgeCount)
	}
}

func TestFlattenMembershipsCycle(t *testing.T) {
	// A -> B -> A cycle must terminate and not emit self-membership.
	groups := []ad.SyncGroup{
		group("A", "CN=A,DC=test", "S-A", "CN=B,DC=test"),
		group("B", "CN=B,DC=test", "S-B", "CN=A,DC=test"),
	}
	users := []ad.SyncUser{user("S-U", "CN=A,DC=test")}

	records, err := FlattenMemberships(users, groups)
	if err != nil {
		t.Fatalf("FlattenMemberships returned error: %v", err)
	}

	edges := indexEdges(t, records)
	requireEdge(t, edges, "S-U", "S-A", true, "A")
	requireEdge(t, edges, "S-U", "S-B", false, "A > B")
	requireEdge(t, edges, "S-A", "S-B", true, "B")
	requireEdge(t, edges, "S-B", "S-A", true, "A")

	for _, record := range records {
		if record.MemberSID == record.GroupSID {
			t.Fatalf("self-membership edge emitted for %s", record.MemberSID)
		}
	}
	if len(records) != 4 {
		t.Fatalf("expected 4 edges, got %d", len(records))
	}
}

func TestFlattenMembershipsSkipsUnknownAndEmpty(t *testing.T) {
	groups := []ad.SyncGroup{
		group("A", "CN=A,DC=test", "S-A", "CN=Unknown,DC=other"),
		group("NoSID", "CN=NoSID,DC=test", "", "CN=A,DC=test"),
	}
	users := []ad.SyncUser{
		user("S-U", "CN=A,DC=test", "CN=Unknown,DC=other"),
		{DN: "CN=nosid,DC=test", SID: "", MemberOf: []string{"CN=A,DC=test"}},
	}

	records, err := FlattenMemberships(users, groups)
	if err != nil {
		t.Fatalf("FlattenMemberships returned error: %v", err)
	}

	edges := indexEdges(t, records)
	requireEdge(t, edges, "S-U", "S-A", true, "A")
	if len(records) != 1 {
		t.Fatalf("expected only 1 edge (unknown DNs and empty SIDs skipped), got %d", len(records))
	}
}
