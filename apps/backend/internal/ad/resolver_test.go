package ad

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/weibinliao/OpenAD/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupResolverResolvesNestedGroupsUpToTenLevels(t *testing.T) {
	rootDN := "CN=Root,OU=Groups,DC=example,DC=com"
	group1DN := "CN=Group01,OU=Groups,DC=example,DC=com"
	group2DN := "CN=Group02,OU=Groups,DC=example,DC=com"
	group3DN := "CN=Group03,OU=Groups,DC=example,DC=com"
	group4DN := "CN=Group04,OU=Groups,DC=example,DC=com"
	group5DN := "CN=Group05,OU=Groups,DC=example,DC=com"
	group6DN := "CN=Group06,OU=Groups,DC=example,DC=com"
	group7DN := "CN=Group07,OU=Groups,DC=example,DC=com"
	group8DN := "CN=Group08,OU=Groups,DC=example,DC=com"
	group9DN := "CN=Group09,OU=Groups,DC=example,DC=com"
	group10DN := "CN=Group10,OU=Groups,DC=example,DC=com"
	group11DN := "CN=Group11,OU=Groups,DC=example,DC=com"
	allowedUserDN := "CN=Allowed User,OU=Users,DC=example,DC=com"
	blockedUserDN := "CN=Blocked User,OU=Users,DC=example,DC=com"

	directory := newFakeGroupDirectory(
		group(rootDN, nestedGroup(group1DN)),
		group(group1DN, nestedGroup(group2DN)),
		group(group2DN, nestedGroup(group3DN)),
		group(group3DN, nestedGroup(group4DN)),
		group(group4DN, nestedGroup(group5DN)),
		group(group5DN, nestedGroup(group6DN)),
		group(group6DN, nestedGroup(group7DN)),
		group(group7DN, nestedGroup(group8DN)),
		group(group8DN, nestedGroup(group9DN)),
		group(group9DN, nestedGroup(group10DN)),
		group(group10DN, user(allowedUserDN), nestedGroup(group11DN)),
		group(group11DN, user(blockedUserDN)),
	)

	resolver := NewGroupResolver(directory, WithMaxDepth(10), WithMembershipCache(NewMembershipCache(time.Minute)))
	resolution, err := resolver.ResolveGroupMembers(context.Background(), rootDN)

	require.NoError(t, err)
	require.Len(t, resolution.Members, 1)
	assert.Equal(t, allowedUserDN, resolution.Members[0].DN)
	assert.Equal(t, 10, resolution.Members[0].Depth)
	assert.Equal(t, []string{rootDN, group1DN, group2DN, group3DN, group4DN, group5DN, group6DN, group7DN, group8DN, group9DN, group10DN}, resolution.Members[0].Path)
	assert.True(t, resolution.MaxDepthReached)
	assert.Empty(t, resolution.Cycles)
	assert.NotContains(t, resolvedMemberDNs(resolution), blockedUserDN)
}

func TestGroupResolverDetectsCircularMembership(t *testing.T) {
	rootDN := "CN=Finance,OU=Groups,DC=example,DC=com"
	approvalDN := "CN=Approvals,OU=Groups,DC=example,DC=com"
	reviewDN := "CN=Reviewers,OU=Groups,DC=example,DC=com"
	rootUserDN := "CN=Root User,OU=Users,DC=example,DC=com"
	nestedUserDN := "CN=Nested User,OU=Users,DC=example,DC=com"

	directory := newFakeGroupDirectory(
		group(rootDN, user(rootUserDN), nestedGroup(approvalDN)),
		group(approvalDN, nestedGroup(reviewDN)),
		group(reviewDN, user(nestedUserDN), nestedGroup(approvalDN)),
	)

	resolver := NewGroupResolver(directory, WithMembershipCache(NewMembershipCache(time.Minute)))
	resolution, err := resolver.ResolveGroupMembers(context.Background(), rootDN)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{rootUserDN, nestedUserDN}, resolvedMemberDNs(resolution))
	require.Len(t, resolution.Cycles, 1)
	assert.Equal(t, approvalDN, resolution.Cycles[0].GroupDN)
	assert.Equal(t, []string{rootDN, approvalDN, reviewDN, approvalDN}, resolution.Cycles[0].Path)
	assert.False(t, resolution.MaxDepthReached)
	assert.Equal(t, 3, directory.TotalCalls())
}

func TestGroupResolverUsesCacheForRepeatedQueries(t *testing.T) {
	rootDN := "CN=Root,OU=Groups,DC=example,DC=com"
	group1DN := "CN=Cached01,OU=Groups,DC=example,DC=com"
	group2DN := "CN=Cached02,OU=Groups,DC=example,DC=com"
	group3DN := "CN=Cached03,OU=Groups,DC=example,DC=com"
	userDN := "CN=Cached User,OU=Users,DC=example,DC=com"

	groups := []models.ADGroup{
		group(rootDN, nestedGroup(group1DN)),
		group(group1DN, nestedGroup(group2DN)),
		group(group2DN, nestedGroup(group3DN)),
		group(group3DN, user(userDN)),
	}

	withoutCacheDirectory := newFakeGroupDirectory(groups...)
	withoutCacheDirectory.latency = 3 * time.Millisecond
	withoutCacheResolver := NewGroupResolver(withoutCacheDirectory, WithMembershipCache(NewMembershipCache(0)))

	withCacheDirectory := newFakeGroupDirectory(groups...)
	withCacheDirectory.latency = 3 * time.Millisecond
	withCacheResolver := NewGroupResolver(withCacheDirectory, WithMembershipCache(NewMembershipCache(time.Minute)))

	withoutCacheDuration := resolveRepeatedly(t, withoutCacheResolver, rootDN, 20)
	withCacheDuration := resolveRepeatedly(t, withCacheResolver, rootDN, 20)

	assert.GreaterOrEqual(t, withoutCacheDirectory.TotalCalls(), withCacheDirectory.TotalCalls()*10)
	assert.GreaterOrEqual(t, withoutCacheDuration, withCacheDuration*10)
}

func TestMembershipCacheStoresAndExpiresEntries(t *testing.T) {
	cache := NewMembershipCache(20 * time.Millisecond)
	groupDN := "CN=CacheTarget,OU=Groups,DC=example,DC=com"
	storedGroup := group(groupDN, user("CN=Cached User,OU=Users,DC=example,DC=com"))

	cache.Set(storedGroup)

	cachedGroup, found := cache.Get("  cn=cachetarget,ou=groups,dc=example,dc=com  ")
	require.True(t, found)
	assert.Equal(t, storedGroup.DN, cachedGroup.DN)
	assert.Len(t, cachedGroup.Members, 1)

	time.Sleep(30 * time.Millisecond)

	_, found = cache.Get(groupDN)
	assert.False(t, found)
}

func TestMembershipCacheDeleteAndClear(t *testing.T) {
	cache := NewMembershipCache(time.Minute)
	firstGroupDN := "CN=DeleteTarget,OU=Groups,DC=example,DC=com"
	secondGroupDN := "CN=ClearTarget,OU=Groups,DC=example,DC=com"

	cache.Set(group(firstGroupDN, user("CN=User01,OU=Users,DC=example,DC=com")))
	cache.Delete(firstGroupDN)

	_, found := cache.Get(firstGroupDN)
	assert.False(t, found)

	cache.Set(group(secondGroupDN, user("CN=User02,OU=Users,DC=example,DC=com")))
	cache.Clear()

	_, found = cache.Get(secondGroupDN)
	assert.False(t, found)
}

func TestGroupResolverReturnsValidationErrorForEmptyDN(t *testing.T) {
	resolver := NewGroupResolver(newFakeGroupDirectory())

	resolution, err := resolver.ResolveGroupMembers(context.Background(), "   ")

	assert.Nil(t, resolution)
	assert.EqualError(t, err, "group distinguished name is required")
}

func TestGroupResolverPropagatesDirectoryErrors(t *testing.T) {
	rootDN := "CN=Broken,OU=Groups,DC=example,DC=com"
	directory := newFakeGroupDirectory(group(rootDN))
	directory.errByDN[normalizeTestDN(rootDN)] = fmt.Errorf("directory unavailable")

	resolver := NewGroupResolver(directory)
	resolution, err := resolver.ResolveGroupMembers(context.Background(), rootDN)

	assert.Nil(t, resolution)
	assert.EqualError(t, err, "directory unavailable")
}

func TestGroupResolverPrefersShortestMembershipPath(t *testing.T) {
	rootDN := "CN=Root,OU=Groups,DC=example,DC=com"
	deepBranchDN := "CN=DeepBranch,OU=Groups,DC=example,DC=com"
	innerBranchDN := "CN=InnerBranch,OU=Groups,DC=example,DC=com"
	shallowBranchDN := "CN=ShallowBranch,OU=Groups,DC=example,DC=com"
	userDN := "CN=Shared User,OU=Users,DC=example,DC=com"

	directory := newFakeGroupDirectory(
		group(rootDN, nestedGroup(deepBranchDN), nestedGroup(shallowBranchDN)),
		group(deepBranchDN, nestedGroup(innerBranchDN)),
		group(innerBranchDN, user(userDN)),
		group(shallowBranchDN, user(userDN)),
	)

	resolver := NewGroupResolver(directory, WithMembershipCache(NewMembershipCache(time.Minute)))
	resolution, err := resolver.ResolveGroupMembers(context.Background(), rootDN)

	require.NoError(t, err)
	require.Len(t, resolution.Members, 1)
	assert.Equal(t, 1, resolution.Members[0].Depth)
	assert.Equal(t, []string{rootDN, shallowBranchDN}, resolution.Members[0].Path)
}

func TestGroupResolverSkipsRepeatedNestedGroupExpansion(t *testing.T) {
	rootDN := "CN=Root,OU=Groups,DC=example,DC=com"
	sharedGroupDN := "CN=SharedGroup,OU=Groups,DC=example,DC=com"
	userDN := "CN=Shared User,OU=Users,DC=example,DC=com"

	directory := newFakeGroupDirectory(
		group(rootDN, nestedGroup(sharedGroupDN), nestedGroup(sharedGroupDN)),
		group(sharedGroupDN, user(userDN)),
	)

	resolver := NewGroupResolver(directory, WithMembershipCache(NewMembershipCache(time.Minute)))
	resolution, err := resolver.ResolveGroupMembers(context.Background(), rootDN)

	require.NoError(t, err)
	require.Len(t, resolution.Members, 1)
	assert.Equal(t, 2, directory.TotalCalls())
}

func TestGroupResolverReturnsErrorWhenDirectoryReturnsNilGroup(t *testing.T) {
	resolver := NewGroupResolver(nilReturningDirectory{})

	resolution, err := resolver.ResolveGroupMembers(context.Background(), "CN=Missing,OU=Groups,DC=example,DC=com")

	assert.Nil(t, resolution)
	assert.EqualError(t, err, "group not found: CN=Missing,OU=Groups,DC=example,DC=com")
}

func resolveRepeatedly(t *testing.T, resolver *GroupResolver, rootDN string, iterations int) time.Duration {
	t.Helper()

	startedAt := time.Now()
	for index := 0; index < iterations; index++ {
		resolution, err := resolver.ResolveGroupMembers(context.Background(), rootDN)
		require.NoError(t, err)
		require.Len(t, resolution.Members, 1)
	}

	return time.Since(startedAt)
}

func resolvedMemberDNs(resolution *models.ADGroupResolution) []string {
	dns := make([]string, 0, len(resolution.Members))
	for _, member := range resolution.Members {
		dns = append(dns, member.DN)
	}

	return dns
}

type fakeGroupDirectory struct {
	latency time.Duration

	mu         sync.Mutex
	groups     map[string]models.ADGroup
	principals map[string]models.ADPrincipal
	calls      map[string]int
	errByDN    map[string]error
}

type nilReturningDirectory struct{}

func (nilReturningDirectory) GetGroup(_ context.Context, _ string) (*models.ADGroup, error) {
	return nil, nil
}

func (nilReturningDirectory) GetPrincipal(_ context.Context, _ string) (*models.ADPrincipal, error) {
	return nil, nil
}

func newFakeGroupDirectory(groups ...models.ADGroup) *fakeGroupDirectory {
	directory := &fakeGroupDirectory{
		groups:     make(map[string]models.ADGroup, len(groups)),
		principals: make(map[string]models.ADPrincipal, len(groups)),
		calls:      make(map[string]int, len(groups)),
		errByDN:    make(map[string]error),
	}

	for _, currentGroup := range groups {
		directory.groups[normalizeTestDN(currentGroup.DN)] = cloneTestGroup(currentGroup)
		directory.principals[normalizeTestDN(currentGroup.DN)] = models.ADPrincipal{
			DN:   currentGroup.DN,
			Name: currentGroup.Name,
			Type: models.ADObjectTypeGroup,
		}

		for _, member := range currentGroup.Members {
			if member.DN == "" {
				continue
			}

			normalizedMemberDN := normalizeTestDN(member.DN)
			if _, found := directory.principals[normalizedMemberDN]; found {
				continue
			}

			directory.principals[normalizedMemberDN] = member
		}
	}

	return directory
}

func (directory *fakeGroupDirectory) GetGroup(_ context.Context, distinguishedName string) (*models.ADGroup, error) {
	if directory.latency > 0 {
		time.Sleep(directory.latency)
	}

	normalizedDN := normalizeTestDN(distinguishedName)

	directory.mu.Lock()
	directory.calls[normalizedDN]++
	err := directory.errByDN[normalizedDN]
	storedGroup, found := directory.groups[normalizedDN]
	directory.mu.Unlock()

	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("group not found: %s", strings.TrimSpace(distinguishedName))
	}

	groupCopy := cloneTestGroup(storedGroup)
	return &groupCopy, nil
}

func (directory *fakeGroupDirectory) GetPrincipal(_ context.Context, distinguishedName string) (*models.ADPrincipal, error) {
	normalizedDN := normalizeTestDN(distinguishedName)

	directory.mu.Lock()
	defer directory.mu.Unlock()

	principal, found := directory.principals[normalizedDN]
	if !found {
		return nil, fmt.Errorf("principal not found: %s", strings.TrimSpace(distinguishedName))
	}

	principalCopy := principal
	return &principalCopy, nil
}

func (directory *fakeGroupDirectory) TotalCalls() int {
	directory.mu.Lock()
	defer directory.mu.Unlock()

	totalCalls := 0
	for _, count := range directory.calls {
		totalCalls += count
	}

	return totalCalls
}

func group(distinguishedName string, members ...models.ADPrincipal) models.ADGroup {
	return models.ADGroup{
		DN:      distinguishedName,
		Name:    distinguishedName,
		Members: members,
	}
}

func user(distinguishedName string) models.ADPrincipal {
	return models.ADPrincipal{
		DN:   distinguishedName,
		Name: distinguishedName,
		Type: models.ADObjectTypeUser,
	}
}

func nestedGroup(distinguishedName string) models.ADPrincipal {
	return models.ADPrincipal{
		DN:   distinguishedName,
		Name: distinguishedName,
		Type: models.ADObjectTypeGroup,
	}
}

func normalizeTestDN(distinguishedName string) string {
	return strings.ToLower(strings.TrimSpace(distinguishedName))
}

func cloneTestGroup(group models.ADGroup) models.ADGroup {
	clonedGroup := group
	if group.Members == nil {
		return clonedGroup
	}

	clonedGroup.Members = append([]models.ADPrincipal(nil), group.Members...)
	return clonedGroup
}
