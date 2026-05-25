package service

import (
	"fmt"
	"sync"

	"github.com/QuantumNous/new-api/model"
)

// orgRateLimitCache stores cached rate limit rules per org
// Key format: "org_type:org_id"
var (
	orgRateLimitCache     = make(map[string][]*model.OrganizationRateLimit)
	orgRateLimitCacheLock sync.RWMutex
)

func orgRateLimitCacheKey(orgType string, orgId int) string {
	return fmt.Sprintf("%s:%d", orgType, orgId)
}

// GetCachedRateLimits returns cached rate limit rules for an org, loading from DB if needed
func GetCachedRateLimits(orgType string, orgId int) ([]*model.OrganizationRateLimit, error) {
	key := orgRateLimitCacheKey(orgType, orgId)

	orgRateLimitCacheLock.RLock()
	rules, ok := orgRateLimitCache[key]
	orgRateLimitCacheLock.RUnlock()

	if ok {
		return rules, nil
	}

	// Load from DB
	dbRules, err := model.GetEnabledRateLimitsByOrg(orgType, orgId)
	if err != nil {
		return nil, err
	}

	orgRateLimitCacheLock.Lock()
	orgRateLimitCache[key] = dbRules
	orgRateLimitCacheLock.Unlock()

	return dbRules, nil
}

// InvalidateRateLimitCache removes cached rules for a specific org
func InvalidateRateLimitCache(orgType string, orgId int) {
	key := orgRateLimitCacheKey(orgType, orgId)

	orgRateLimitCacheLock.Lock()
	delete(orgRateLimitCache, key)
	orgRateLimitCacheLock.Unlock()
}

// ClearAllRateLimitCache clears the entire rate limit cache
func ClearAllRateLimitCache() {
	orgRateLimitCacheLock.Lock()
	orgRateLimitCache = make(map[string][]*model.OrganizationRateLimit)
	orgRateLimitCacheLock.Unlock()
}
