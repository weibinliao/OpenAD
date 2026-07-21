package adsync

import (
	"fmt"
	"strings"

	"github.com/weibinliao/OpenAD/internal/ad"
	"github.com/weibinliao/OpenAD/internal/models"
)

// MaxMembershipEdges caps the number of flattened membership edges one sync
// run may produce; flattening aborts with ErrTooManyMemberships beyond it.
const MaxMembershipEdges = 2_000_000

// ErrTooManyMemberships is returned when flattening exceeds MaxMembershipEdges.
var ErrTooManyMemberships = fmt.Errorf("flattened membership edge count exceeds the maximum of %d; narrow the sync scope (base DN) and retry", MaxMembershipEdges)

// FlattenMemberships computes the transitive closure of group membership for
// every user (user -> each direct and nested group) and for every group
// (group -> each transitive parent group), so downstream analysis can join
// against either side without expanding nesting at query time.
//
// Records are returned without RunID; the caller stamps it before insert.
// Group DNs referenced in memberOf but absent from groups are skipped
// (foreign-domain or filtered objects). Cycles are tolerated: traversal is
// breadth-first with a per-origin visited set, so each reachable group is
// recorded once via its shortest chain.
func FlattenMemberships(users []ad.SyncUser, groups []ad.SyncGroup) ([]models.ADMembershipRecord, error) {
	groupsByDN := make(map[string]*ad.SyncGroup, len(groups))
	for index := range groups {
		group := &groups[index]
		if strings.TrimSpace(group.DN) == "" {
			continue
		}
		groupsByDN[group.DN] = group
	}

	records := make([]models.ADMembershipRecord, 0, len(users)+len(groups))

	appendEdge := func(memberSID, groupSID string, direct bool, chain []string) error {
		if memberSID == "" || groupSID == "" {
			return nil
		}
		if len(records) >= MaxMembershipEdges {
			return ErrTooManyMemberships
		}
		records = append(records, models.ADMembershipRecord{
			MemberSID: memberSID,
			GroupSID:  groupSID,
			Direct:    direct,
			ViaChain:  strings.Join(chain, " > "),
		})
		return nil
	}

	// flattenFrom walks upward from the given direct memberOf DNs, emitting
	// one edge per reachable group for the origin principal.
	flattenFrom := func(originSID, originDN string, directMemberOf []string) error {
		type queueItem struct {
			group *ad.SyncGroup
			chain []string
		}

		visited := make(map[string]struct{}, len(directMemberOf))
		if originDN != "" {
			// A group never records membership of itself, even via a cycle.
			visited[originDN] = struct{}{}
		}
		queue := make([]queueItem, 0, len(directMemberOf))

		for _, groupDN := range directMemberOf {
			group, known := groupsByDN[groupDN]
			if !known {
				continue
			}
			if _, seen := visited[group.DN]; seen {
				continue
			}
			visited[group.DN] = struct{}{}

			chain := []string{group.Name}
			if err := appendEdge(originSID, group.SID, true, chain); err != nil {
				return err
			}
			queue = append(queue, queueItem{group: group, chain: chain})
		}

		for len(queue) > 0 {
			item := queue[0]
			queue = queue[1:]

			for _, parentDN := range item.group.MemberOf {
				parent, known := groupsByDN[parentDN]
				if !known {
					continue
				}
				if _, seen := visited[parent.DN]; seen {
					continue
				}
				visited[parent.DN] = struct{}{}

				chain := make([]string, 0, len(item.chain)+1)
				chain = append(chain, item.chain...)
				chain = append(chain, parent.Name)
				if err := appendEdge(originSID, parent.SID, false, chain); err != nil {
					return err
				}
				queue = append(queue, queueItem{group: parent, chain: chain})
			}
		}

		return nil
	}

	for index := range users {
		user := &users[index]
		if user.SID == "" {
			continue
		}
		if err := flattenFrom(user.SID, "", user.MemberOf); err != nil {
			return nil, err
		}
	}

	for index := range groups {
		group := &groups[index]
		if group.SID == "" {
			continue
		}
		if err := flattenFrom(group.SID, group.DN, group.MemberOf); err != nil {
			return nil, err
		}
	}

	return records, nil
}
