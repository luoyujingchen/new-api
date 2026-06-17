package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
)

// UserBase struct remains the same as it represents the cached data structure
type UserBase struct {
	Id                  int    `json:"id"`
	Group               string `json:"group"`
	Email               string `json:"email"`
	Quota               int    `json:"quota"`
	Status              int    `json:"status"`
	Username            string `json:"username"`
	DisplayName         string `json:"display_name"`
	Role                int    `json:"role"`
	Setting             string `json:"setting"`
	CompanyId           int64  `json:"company_id"`
	CompanyName         string `json:"company_name"`
	CompanyCode         string `json:"company_code"`
	DepartmentId        int64  `json:"department_id"`
	DepartmentName      string `json:"department_name"`
	DepartmentPath      string `json:"department_path"`
	DepartmentLevel     int    `json:"department_level"`
	DepartmentHierarchy string `json:"department_hierarchy"`
	CompanyLoaded       bool   `json:"company_loaded"`
	OrganizationLoaded  bool   `json:"organization_loaded"`
}

func (user *UserBase) WriteContext(c *gin.Context) {
	common.SetContextKey(c, constant.ContextKeyUserGroup, user.Group)
	common.SetContextKey(c, constant.ContextKeyUserQuota, user.Quota)
	common.SetContextKey(c, constant.ContextKeyUserStatus, user.Status)
	common.SetContextKey(c, constant.ContextKeyUserEmail, user.Email)
	common.SetContextKey(c, constant.ContextKeyUserName, user.Username)
	common.SetContextKey(c, constant.ContextKeyUserDisplayName, user.DisplayName)
	common.SetContextKey(c, constant.ContextKeyUserRole, user.Role)
	setting := user.GetSetting()
	common.SetContextKey(c, constant.ContextKeyUserSetting, setting)
	common.SetContextKey(c, constant.ContextKeyRecordIpLog, setting.RecordIpLog)
	if user.CompanyLoaded && user.CompanyId > 0 {
		common.SetContextKey(c, constant.ContextKeyUserCompanyId, user.CompanyId)
	}
	if user.OrganizationLoaded {
		common.SetContextKey(c, constant.ContextKeyUserCompanyName, user.CompanyName)
		common.SetContextKey(c, constant.ContextKeyUserCompanyCode, user.CompanyCode)
		if user.DepartmentId > 0 {
			common.SetContextKey(c, constant.ContextKeyUserDepartmentId, user.DepartmentId)
		}
		common.SetContextKey(c, constant.ContextKeyUserDepartmentName, user.DepartmentName)
		common.SetContextKey(c, constant.ContextKeyUserDepartmentPath, user.DepartmentPath)
		common.SetContextKey(c, constant.ContextKeyUserDepartmentLevel, user.DepartmentLevel)
		common.SetContextKey(c, constant.ContextKeyUserDepartmentHierarchy, user.DepartmentHierarchy)
	}
}

func (user *UserBase) GetSetting() dto.UserSetting {
	setting := dto.UserSetting{}
	if user.Setting != "" {
		err := common.Unmarshal([]byte(user.Setting), &setting)
		if err != nil {
			common.SysLog("failed to unmarshal setting: " + err.Error())
		}
	}
	return setting
}

// getUserCacheKey returns the key for user cache
func getUserCacheKey(userId int) string {
	return fmt.Sprintf("user:%d", userId)
}

// invalidateUserCache clears user cache
func invalidateUserCache(userId int) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisDelKey(getUserCacheKey(userId))
}

// InvalidateUserCache is the exported version of invalidateUserCache.
// 供 controller 等上层包在用户状态变更（如禁用、删除、角色变更）后主动清理缓存。
func InvalidateUserCache(userId int) error {
	return invalidateUserCache(userId)
}

// updateUserCache updates all user cache fields using hash
func updateUserCache(user User) error {
	if !common.RedisEnabled {
		return nil
	}
	baseUser := user.ToBaseUser()
	enrichUserBaseOrganization(baseUser)

	return common.RedisHSetObj(
		getUserCacheKey(user.Id),
		baseUser,
		time.Duration(common.RedisKeyCacheSeconds())*time.Second,
	)
}

// GetUserCache gets complete user cache from hash
func GetUserCache(userId int) (userCache *UserBase, err error) {
	var user *User
	var fromDB bool
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) && user != nil {
			gopool.Go(func() {
				if err := updateUserCache(*user); err != nil {
					common.SysLog("failed to update user status cache: " + err.Error())
				}
			})
		}
	}()

	// Try getting from Redis first
	userCache, err = cacheGetUserBase(userId)
	if err == nil && userCache.CompanyLoaded && userCache.OrganizationLoaded {
		return userCache, nil
	}

	// If Redis fails, get from DB
	fromDB = true
	user, err = GetUserById(userId, false)
	if err != nil {
		return nil, err // Return nil and error if DB lookup fails
	}

	// Create cache object from user data
	userCache = &UserBase{
		Id:            user.Id,
		Group:         user.Group,
		Quota:         user.Quota,
		Status:        user.Status,
		Username:      user.Username,
		DisplayName:   user.DisplayName,
		Role:          user.Role,
		Setting:       user.Setting,
		Email:         user.Email,
		CompanyLoaded: true,
	}
	if user.CompanyId != nil {
		userCache.CompanyId = *user.CompanyId
	}
	if user.DepartmentId != nil {
		userCache.DepartmentId = *user.DepartmentId
	}
	enrichUserBaseOrganization(userCache)

	return userCache, nil
}

func enrichUserBaseOrganization(userCache *UserBase) {
	if userCache == nil {
		return
	}
	userCache.OrganizationLoaded = true
	if userCache.CompanyId > 0 {
		if company, err := GetCompanyByID(userCache.CompanyId); err == nil && company != nil {
			userCache.CompanyName = company.Name
			userCache.CompanyCode = company.Code
		}
	}
	if userCache.DepartmentId > 0 {
		if department, err := GetDepartmentByID(userCache.DepartmentId); err == nil && department != nil {
			userCache.DepartmentName = department.Name
			userCache.DepartmentPath = department.Path
			userCache.DepartmentLevel = department.Level
			if hierarchyBytes, err := common.Marshal(buildDepartmentHierarchy(department)); err == nil {
				userCache.DepartmentHierarchy = string(hierarchyBytes)
			}
			if userCache.CompanyId == 0 {
				userCache.CompanyId = department.CompanyId
				userCache.CompanyLoaded = true
			}
			if userCache.CompanyName == "" && department.Company.Id > 0 {
				userCache.CompanyName = department.Company.Name
				userCache.CompanyCode = department.Company.Code
			}
		}
	}
}

func buildDepartmentHierarchy(department *Department) []map[string]interface{} {
	if department == nil {
		return nil
	}
	if strings.TrimSpace(department.Path) == "" {
		return []map[string]interface{}{{
			"id":    department.Id,
			"name":  department.Name,
			"level": department.Level,
		}}
	}
	parts := strings.Split(strings.Trim(department.Path, "/"), "/")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(part, 10, 64)
		if err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var departments []Department
	if err := DB.Where("id IN ?", ids).Find(&departments).Error; err != nil {
		return nil
	}
	byID := make(map[int64]Department, len(departments))
	for _, item := range departments {
		byID[item.Id] = item
	}
	hierarchy := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		item, ok := byID[id]
		if !ok {
			continue
		}
		hierarchy = append(hierarchy, map[string]interface{}{
			"id":    item.Id,
			"name":  item.Name,
			"level": item.Level,
		})
	}
	return hierarchy
}

func cacheGetUserBase(userId int) (*UserBase, error) {
	if !common.RedisEnabled {
		return nil, fmt.Errorf("redis is not enabled")
	}
	var userCache UserBase
	// Try getting from Redis first
	err := common.RedisHGetObj(getUserCacheKey(userId), &userCache)
	if err != nil {
		return nil, err
	}
	return &userCache, nil
}

// Add atomic quota operations using hash fields
func cacheIncrUserQuota(userId int, delta int64) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHIncrBy(getUserCacheKey(userId), "Quota", delta)
}

func cacheDecrUserQuota(userId int, delta int64) error {
	return cacheIncrUserQuota(userId, -delta)
}

// Helper functions to get individual fields if needed
func getUserGroupCache(userId int) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Group, nil
}

func getUserQuotaCache(userId int) (int, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return 0, err
	}
	return cache.Quota, nil
}

func getUserStatusCache(userId int) (int, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return 0, err
	}
	return cache.Status, nil
}

func getUserNameCache(userId int) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Username, nil
}

func getUserSettingCache(userId int) (dto.UserSetting, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return dto.UserSetting{}, err
	}
	return cache.GetSetting(), nil
}

// New functions for individual field updates
func updateUserStatusCache(userId int, status bool) error {
	if !common.RedisEnabled {
		return nil
	}
	statusInt := common.UserStatusEnabled
	if !status {
		statusInt = common.UserStatusDisabled
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Status", fmt.Sprintf("%d", statusInt))
}

func updateUserQuotaCache(userId int, quota int) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Quota", fmt.Sprintf("%d", quota))
}

func updateUserGroupCache(userId int, group string) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Group", group)
}

func UpdateUserGroupCache(userId int, group string) error {
	return updateUserGroupCache(userId, group)
}

func updateUserNameCache(userId int, username string) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Username", username)
}

func updateUserSettingCache(userId int, setting string) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Setting", setting)
}

// GetUserLanguage returns the user's language preference from cache
// Uses the existing GetUserCache mechanism for efficiency
func GetUserLanguage(userId int) string {
	userCache, err := GetUserCache(userId)
	if err != nil {
		return ""
	}
	return userCache.GetSetting().Language
}
