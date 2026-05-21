package service

import (
	"github.com/QuantumNous/new-api/model"
	"sync"
	"time"
)

var (
	ruleCache     = make(map[string][]*model.OrganizationRateLimit)
	ruleCacheMu   sync.RWMutex
	cacheTTL      = 5 * time.Minute
	lastCacheTime = make(map[string]time.Time)
)

// getCacheKey 生成缓存键
func getCacheKey(orgType string, orgId int64) string {
	return orgType + ":" + formatInt64(orgId)
}

// GetCachedRules 获取缓存的规则
func GetCachedRules(orgType string, orgId int64) ([]*model.OrganizationRateLimit, error) {
	cacheKey := getCacheKey(orgType, orgId)

	ruleCacheMu.RLock()
	if rules, ok := ruleCache[cacheKey]; ok {
		if cacheTime, exists := lastCacheTime[cacheKey]; exists && time.Since(cacheTime) < cacheTTL {
			ruleCacheMu.RUnlock()
			return rules, nil
		}
	}
	ruleCacheMu.RUnlock()

	// 缓存过期或不存在，从数据库加载
	ruleCacheMu.Lock()
	defer ruleCacheMu.Unlock()

	// 双重检查
	if rules, ok := ruleCache[cacheKey]; ok {
		if cacheTime, exists := lastCacheTime[cacheKey]; exists && time.Since(cacheTime) < cacheTTL {
			return rules, nil
		}
	}

	// 从数据库加载
	rules, err := loadRulesFromDB(orgType, orgId)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	ruleCache[cacheKey] = rules
	lastCacheTime[cacheKey] = time.Now()

	return rules, nil
}

// loadRulesFromDB 从数据库加载规则
func loadRulesFromDB(orgType string, orgId int64) ([]*model.OrganizationRateLimit, error) {
	enabled := 1
	return model.GetOrganizationRateLimits(orgType, orgId, &enabled)
}

// InvalidateCache 清除缓存（创建/更新/删除规则时调用）
func InvalidateOrgRateLimitCache(orgType string, orgId int64) {
	cacheKey := getCacheKey(orgType, orgId)

	ruleCacheMu.Lock()
	defer ruleCacheMu.Unlock()

	delete(ruleCache, cacheKey)
	delete(lastCacheTime, cacheKey)
}

// InvalidateAllCache 清除所有缓存（用于测试或批量更新）
func InvalidateAllOrgRateLimitCache() {
	ruleCacheMu.Lock()
	defer ruleCacheMu.Unlock()

	ruleCache = make(map[string][]*model.OrganizationRateLimit)
	lastCacheTime = make(map[string]time.Time)
}

// GetCacheStats 获取缓存统计信息（用于调试）
func GetOrgRateLimitCacheStats() map[string]interface{} {
	ruleCacheMu.RLock()
	defer ruleCacheMu.RUnlock()

	stats := make(map[string]interface{})
	stats["cached_entries"] = len(ruleCache)
	stats["cache_ttl_minutes"] = cacheTTL.Minutes()

	entries := make([]map[string]interface{}, 0, len(ruleCache))
	for key, rules := range ruleCache {
		if cacheTime, exists := lastCacheTime[key]; exists {
			entries = append(entries, map[string]interface{}{
				"key":         key,
				"rule_count":  len(rules),
				"cached_at":   cacheTime,
				"age_seconds": time.Since(cacheTime).Seconds(),
			})
		}
	}
	stats["entries"] = entries

	return stats
}
