package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// OrganizationRateLimitService 组织限流服务
type OrganizationRateLimitService struct{}

var (
	rateLimitService     *OrganizationRateLimitService
	rateLimitServiceOnce sync.Once
)

// GetOrganizationRateLimitService 获取组织限流服务实例
func GetOrganizationRateLimitService() *OrganizationRateLimitService {
	rateLimitServiceOnce.Do(func() {
		rateLimitService = &OrganizationRateLimitService{}
	})
	return rateLimitService
}

// EffectiveRateLimit 有效限流规则
type EffectiveRateLimit struct {
	Source   string            `json:"source"`   // "department", "company", "group", "global"
	OrgName  string            `json:"org_name,omitempty"`
	OrgType  string            `json:"org_type,omitempty"`
	OrgId    int64             `json:"org_id,omitempty"`
	ModelId  *int64            `json:"model_id"`
	ModelName string           `json:"model_name,omitempty"`
	TimeSlot *model.TimeSlot   `json:"time_slot,omitempty"`
	Rpm      int               `json:"rpm"`
}

// Create 创建组织限流规则
func (s *OrganizationRateLimitService) Create(rule *model.OrganizationRateLimit) error {
	if err := ensureOrganizationRateLimitModelReference(rule); err != nil {
		return err
	}

	// 验证规则
	if err := rule.Validate(); err != nil {
		return err
	}

	// 插入数据库
	if err := rule.Insert(); err != nil {
		return err
	}

	// 清除缓存
	InvalidateOrgRateLimitCache(rule.OrgType, rule.OrgId)

	return nil
}

// Update 更新组织限流规则
func (s *OrganizationRateLimitService) Update(rule *model.OrganizationRateLimit) error {
	if err := ensureOrganizationRateLimitModelReference(rule); err != nil {
		return err
	}

	// 验证规则
	if err := rule.Validate(); err != nil {
		return err
	}

	// 更新数据库
	if err := rule.Update(); err != nil {
		return err
	}

	// 清除缓存
	InvalidateOrgRateLimitCache(rule.OrgType, rule.OrgId)

	return nil
}

// Delete 删除组织限流规则
func (s *OrganizationRateLimitService) Delete(id int64) error {
	// 先获取规则以便清除缓存
	rule, err := model.GetOrganizationRateLimitByID(id)
	if err != nil {
		return err
	}

	// 删除
	if err := model.DeleteOrganizationRateLimitByID(id); err != nil {
		return err
	}

	// 清除缓存
	InvalidateOrgRateLimitCache(rule.OrgType, rule.OrgId)

	return nil
}

// GetByID 获取规则详情
func (s *OrganizationRateLimitService) GetByID(id int64) (*model.OrganizationRateLimit, error) {
	return model.GetOrganizationRateLimitByID(id)
}

// List 获取规则列表

func (s *OrganizationRateLimitService) List(orgType string, orgId int64, modelId *int64, status *int) ([]*model.OrganizationRateLimit, error) {
	if modelId != nil {
		return model.GetOrganizationRateLimitByModel(orgType, orgId, modelId, status)
	}
	return model.GetOrganizationRateLimits(orgType, orgId, status)
}

func ensureOrganizationRateLimitModelReference(rule *model.OrganizationRateLimit) error {
	if rule.ModelName != nil {
		modelName := strings.TrimSpace(*rule.ModelName)
		if modelName == "" {
			rule.ModelName = nil
		} else {
			rule.ModelName = &modelName
		}
	}

	if rule.ModelId != nil {
		modelMeta, err := model.GetModelByID(*rule.ModelId)
		if err != nil {
			return err
		}
		if rule.ModelName == nil {
			modelName := modelMeta.ModelName
			rule.ModelName = &modelName
			return nil
		}
		if modelMeta.ModelName != *rule.ModelName {
			return gorm.ErrInvalidData
		}
		return nil
	}

	if rule.ModelName == nil {
		return nil
	}

	modelMeta, err := model.GetModelByName(*rule.ModelName)
	if err == nil && modelMeta != nil {
		modelID := int64(modelMeta.Id)
		rule.ModelId = &modelID
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}

// GetEffectiveRateLimit 获取用户的有效限流规则
// 优先级：部门规则 > 公司规则 > 分组限流 > 全局限流
func (s *OrganizationRateLimitService) GetEffectiveRateLimit(userId int, modelName string, currentTime time.Time) (*EffectiveRateLimit, error) {
	// 获取用户信息
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return nil, err
	}

	// 1. 检查部门规则（包括祖先部门）
	if user.DepartmentId != nil && *user.DepartmentId > 0 {
		deptLimit, err := s.getDepartmentLimit(*user.DepartmentId, modelName, currentTime)
		if err == nil && deptLimit != nil && deptLimit.Rpm > 0 {
			return deptLimit, nil
		}
	}

	// 2. 检查公司规则
	if user.CompanyId != nil && *user.CompanyId > 0 {
		companyLimit, err := s.getCompanyLimit(*user.CompanyId, modelName, currentTime)
		if err == nil && companyLimit != nil && companyLimit.Rpm > 0 {
			return companyLimit, nil
		}
	}

	// 3. 无组织规则，返回 nil（将使用默认的分组/全局限流）
	return nil, nil
}

// getDepartmentLimit 获取部门限流规则（包括祖先部门）
func (s *OrganizationRateLimitService) getDepartmentLimit(departmentId int64, modelName string, currentTime time.Time) (*EffectiveRateLimit, error) {
	// 获取部门及所有祖先部门
	ancestors, err := model.GetAncestors(departmentId)
	if err != nil {
		return nil, err
	}

	// 构建部门 ID 列表（包括当前部门和所有祖先）
	deptIds := []int64{departmentId}
	for i := len(ancestors) - 1; i >= 0; i-- {
		deptIds = append(deptIds, ancestors[i].Id)
	}

	for _, deptId := range deptIds {
		deptRules, err := GetCachedRules("department", deptId)
		if err != nil {
			continue
		}
		rule, slotIdx := matchRuleByModelAndTime(deptRules, modelName, currentTime)
		if rule != nil {
			return buildEffectiveRateLimit(rule, slotIdx, "department"), nil
		}
	}

	return nil, nil
}

// getCompanyLimit 获取公司限流规则
func (s *OrganizationRateLimitService) getCompanyLimit(companyId int64, modelName string, currentTime time.Time) (*EffectiveRateLimit, error) {
	// 获取公司规则
	rules, err := GetCachedRules("company", companyId)
	if err != nil {
		return nil, err
	}

	rule, slotIdx := matchRuleByModelAndTime(rules, modelName, currentTime)
	if rule != nil {
		return buildEffectiveRateLimit(rule, slotIdx, "company"), nil
	}

	return nil, nil
}

func matchRuleByModelAndTime(rules []*model.OrganizationRateLimit, modelName string, currentTime time.Time) (*model.OrganizationRateLimit, int) {
	requestModelName := strings.TrimSpace(modelName)

	for _, exactMatch := range []bool{true, false} {
		for _, rule := range rules {
			ruleModelName := ""
			if rule.ModelName != nil {
				ruleModelName = strings.TrimSpace(*rule.ModelName)
			}

			if exactMatch {
				if requestModelName == "" || ruleModelName == "" || ruleModelName != requestModelName {
					continue
				}
			} else if ruleModelName != "" {
				continue
			}

			slotIdx := rule.TimeSlots.MatchTimeSlot(currentTime)
			if slotIdx >= 0 && slotIdx < len(rule.Rpms) && rule.Rpms[slotIdx] > 0 {
				return rule, slotIdx
			}
		}
	}
	return nil, -1
}

func buildEffectiveRateLimit(rule *model.OrganizationRateLimit, slotIdx int, source string) *EffectiveRateLimit {
	effective := &EffectiveRateLimit{
		Source:    source,
		OrgType:   rule.OrgType,
		OrgId:     rule.OrgId,
		ModelId:   rule.ModelId,
		TimeSlot:  &rule.TimeSlots[slotIdx],
		Rpm:       rule.Rpms[slotIdx],
	}
	if rule.ModelName != nil {
		effective.ModelName = *rule.ModelName
	}

	if source == "department" {
		if dept, err := model.GetDepartmentByID(rule.OrgId); err == nil {
			effective.OrgName = dept.Name
		}
		return effective
	}

	if company, err := model.GetCompanyByID(rule.OrgId); err == nil {
		effective.OrgName = company.Name
	}
	return effective
}

// GetRedisKey 获取组织限流的 Redis 键
func (s *OrganizationRateLimitService) GetRedisKey(orgType string, orgId int64, modelName string, keyType string) string {
	trimmedModelName := strings.TrimSpace(modelName)
	if trimmedModelName != "" {
		return "rateLimit:" + orgType + ":" + formatInt64(orgId) + ":model:" + trimmedModelName + ":" + keyType
	}
	return "rateLimit:" + orgType + ":" + formatInt64(orgId) + ":" + keyType
}

// GetDeptUsers 获取部门下的所有用户ID
func (s *OrganizationRateLimitService) GetDeptUsers(departmentId int64) ([]int, error) {
	var userIds []int
	err := model.DB.Model(&model.User{}).Where("department_id = ?", departmentId).Pluck("id", &userIds).Error
	return userIds, err
}

// GetCompanyUsers 获取公司下的所有用户ID
func (s *OrganizationRateLimitService) GetCompanyUsers(companyId int64) ([]int, error) {
	var userIds []int
	err := model.DB.Model(&model.User{}).Where("company_id = ?", companyId).Pluck("id", &userIds).Error
	return userIds, err
}

// formatInt64 格式化 int64 为字符串
func formatInt64(n int64) string {
	return fmt.Sprintf("%d", n)
}
