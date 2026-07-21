package ad

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/weibinliao/OpenAD/internal/models"
)

const defaultMaxNestedGroupDepth = 10

type GroupDirectory interface {
	GetGroup(ctx context.Context, distinguishedName string) (*models.ADGroup, error)
	GetPrincipal(ctx context.Context, distinguishedName string) (*models.ADPrincipal, error)
}

type GroupResolver struct {
	directory GroupDirectory
	cache     *MembershipCache
	maxDepth  int
	filter    *ExclusionFilter
}

type GroupResolverOption func(*GroupResolver)

type resolutionState struct {
	memberIndex    map[string]int
	nestedGroups   map[string]string
	expandedGroups map[string]int
	cycleKeys      map[string]struct{}

	members          []models.ADResolvedMember
	cycles           []models.ADGroupCycle
	directoryLookups int
	cacheHits        int
	maxDepthReached  bool
}

func NewGroupResolver(directory GroupDirectory, options ...GroupResolverOption) *GroupResolver {
	resolver := &GroupResolver{
		directory: directory,
		cache:     NewMembershipCache(5 * time.Minute),
		maxDepth:  defaultMaxNestedGroupDepth,
	}

	for _, option := range options {
		if option != nil {
			option(resolver)
		}
	}

	if resolver.maxDepth < 1 {
		resolver.maxDepth = defaultMaxNestedGroupDepth
	}

	return resolver
}

func WithMembershipCache(cache *MembershipCache) GroupResolverOption {
	return func(resolver *GroupResolver) {
		resolver.cache = cache
	}
}

func WithMaxDepth(maxDepth int) GroupResolverOption {
	return func(resolver *GroupResolver) {
		resolver.maxDepth = maxDepth
	}
}

func WithExclusionFilter(filter *ExclusionFilter) GroupResolverOption {
	return func(resolver *GroupResolver) {
		resolver.filter = filter
	}
}

func (resolver *GroupResolver) ResolveGroupMembers(ctx context.Context, distinguishedName string) (*models.ADGroupResolution, error) {
	groupDN := strings.TrimSpace(distinguishedName)
	if groupDN == "" {
		return nil, fmt.Errorf("group distinguished name is required")
	}

	if resolver == nil || resolver.directory == nil {
		return nil, fmt.Errorf("group directory is required")
	}

	state := &resolutionState{
		memberIndex:    make(map[string]int),
		nestedGroups:   make(map[string]string),
		expandedGroups: make(map[string]int),
		cycleKeys:      make(map[string]struct{}),
	}

	if err := resolver.resolveGroup(ctx, groupDN, 0, []string{groupDN}, make(map[string]struct{}), state); err != nil {
		return nil, err
	}

	nestedGroups := make([]string, 0, len(state.nestedGroups))
	for _, nestedGroup := range state.nestedGroups {
		nestedGroups = append(nestedGroups, nestedGroup)
	}
	sort.Strings(nestedGroups)

	return &models.ADGroupResolution{
		GroupDN:          groupDN,
		Members:          state.members,
		NestedGroups:     nestedGroups,
		Cycles:           state.cycles,
		MaxDepthReached:  state.maxDepthReached,
		DirectoryLookups: state.directoryLookups,
		CacheHits:        state.cacheHits,
	}, nil
}

func (resolver *GroupResolver) resolveGroup(ctx context.Context, distinguishedName string, depth int, path []string, active map[string]struct{}, state *resolutionState) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	normalizedGroupDN := normalizeDistinguishedName(distinguishedName)
	if previousDepth, found := state.expandedGroups[normalizedGroupDN]; found && previousDepth <= depth {
		return nil
	}
	state.expandedGroups[normalizedGroupDN] = depth

	active[normalizedGroupDN] = struct{}{}
	defer delete(active, normalizedGroupDN)

	group, fromCache, err := resolver.loadGroup(ctx, distinguishedName)
	if err != nil {
		return err
	}

	if fromCache {
		state.cacheHits++
	} else {
		state.directoryLookups++
	}

	for _, member := range group.Members {
		memberDN := strings.TrimSpace(member.DN)
		if memberDN == "" {
			continue
		}

		if member.Type == models.ADObjectTypeGroup {
			if resolver.filter != nil && resolver.filter.ShouldExcludeGroup(member) {
				continue
			}

			normalizedMemberDN := normalizeDistinguishedName(memberDN)
			state.nestedGroups[normalizedMemberDN] = memberDN

			if _, found := active[normalizedMemberDN]; found {
				state.addCycle(memberDN, extendPath(path, memberDN))
				continue
			}

			if depth+1 > resolver.maxDepth {
				state.maxDepthReached = true
				continue
			}

			if err := resolver.resolveGroup(ctx, memberDN, depth+1, extendPath(path, memberDN), active, state); err != nil {
				return err
			}

			continue
		}

		if resolver.filter != nil && resolver.filter.ShouldExcludeUser(member) {
			continue
		}

		state.addMember(models.ADResolvedMember{
			ADPrincipal: member,
			Depth:       depth,
			Path:        clonePath(path),
		})
	}

	return nil
}

func (resolver *GroupResolver) loadGroup(ctx context.Context, distinguishedName string) (models.ADGroup, bool, error) {
	if resolver.cache != nil {
		cachedGroup, found := resolver.cache.Get(distinguishedName)
		if found {
			return cachedGroup, true, nil
		}
	}

	group, err := resolver.directory.GetGroup(ctx, distinguishedName)
	if err != nil {
		return models.ADGroup{}, false, err
	}
	if group == nil {
		return models.ADGroup{}, false, fmt.Errorf("group not found: %s", strings.TrimSpace(distinguishedName))
	}

	resolvedGroup := cloneADGroup(*group)
	if resolver.cache != nil {
		resolver.cache.Set(resolvedGroup)
	}

	return resolvedGroup, false, nil
}

func (state *resolutionState) addMember(member models.ADResolvedMember) {
	normalizedMemberDN := normalizeDistinguishedName(member.DN)
	if existingIndex, found := state.memberIndex[normalizedMemberDN]; found {
		existingMember := state.members[existingIndex]
		if member.Depth < existingMember.Depth {
			state.members[existingIndex] = cloneResolvedMember(member)
		}
		return
	}

	state.memberIndex[normalizedMemberDN] = len(state.members)
	state.members = append(state.members, cloneResolvedMember(member))
}

func (state *resolutionState) addCycle(groupDN string, path []string) {
	cycleKey := normalizeDistinguishedName(strings.Join(path, "|"))
	if _, found := state.cycleKeys[cycleKey]; found {
		return
	}

	state.cycleKeys[cycleKey] = struct{}{}
	state.cycles = append(state.cycles, models.ADGroupCycle{
		GroupDN: groupDN,
		Path:    clonePath(path),
	})
}

func clonePath(path []string) []string {
	return append([]string(nil), path...)
}

func extendPath(path []string, distinguishedName string) []string {
	clonedPath := clonePath(path)
	return append(clonedPath, distinguishedName)
}

func cloneResolvedMember(member models.ADResolvedMember) models.ADResolvedMember {
	clonedMember := member
	clonedMember.Path = append([]string(nil), member.Path...)
	return clonedMember
}
