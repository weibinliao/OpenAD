package ad

import (
	"strings"
	"sync"
	"time"

	"github.com/weibinliao/OpenAD/internal/models"
)

type MembershipCache struct {
	ttl time.Duration

	mu      sync.RWMutex
	entries map[string]membershipCacheEntry
}

type membershipCacheEntry struct {
	group     models.ADGroup
	expiresAt time.Time
}

func NewMembershipCache(ttl time.Duration) *MembershipCache {
	return &MembershipCache{
		ttl:     ttl,
		entries: make(map[string]membershipCacheEntry),
	}
}

func (cache *MembershipCache) Get(distinguishedName string) (models.ADGroup, bool) {
	if cache == nil || !cache.enabled() {
		return models.ADGroup{}, false
	}

	key := normalizeDistinguishedName(distinguishedName)
	if key == "" {
		return models.ADGroup{}, false
	}

	cache.mu.RLock()
	entry, found := cache.entries[key]
	cache.mu.RUnlock()
	if !found {
		return models.ADGroup{}, false
	}

	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		cache.mu.Lock()
		delete(cache.entries, key)
		cache.mu.Unlock()
		return models.ADGroup{}, false
	}

	return cloneADGroup(entry.group), true
}

func (cache *MembershipCache) Set(group models.ADGroup) {
	if cache == nil || !cache.enabled() {
		return
	}

	key := normalizeDistinguishedName(group.DN)
	if key == "" {
		return
	}

	entry := membershipCacheEntry{
		group: cloneADGroup(group),
	}
	if cache.ttl > 0 {
		entry.expiresAt = time.Now().Add(cache.ttl)
	}

	cache.mu.Lock()
	cache.entries[key] = entry
	cache.mu.Unlock()
}

func (cache *MembershipCache) Delete(distinguishedName string) {
	if cache == nil {
		return
	}

	key := normalizeDistinguishedName(distinguishedName)
	if key == "" {
		return
	}

	cache.mu.Lock()
	delete(cache.entries, key)
	cache.mu.Unlock()
}

func (cache *MembershipCache) Clear() {
	if cache == nil {
		return
	}

	cache.mu.Lock()
	cache.entries = make(map[string]membershipCacheEntry)
	cache.mu.Unlock()
}

func (cache *MembershipCache) enabled() bool {
	return cache != nil && cache.ttl > 0
}

func normalizeDistinguishedName(distinguishedName string) string {
	return strings.ToLower(strings.TrimSpace(distinguishedName))
}

func cloneADGroup(group models.ADGroup) models.ADGroup {
	clonedGroup := group
	if group.Members == nil {
		return clonedGroup
	}

	clonedGroup.Members = append([]models.ADPrincipal(nil), group.Members...)
	return clonedGroup
}
