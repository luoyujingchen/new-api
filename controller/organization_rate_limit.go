package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var orgRateLimitService = service.GetOrganizationRateLimitService()

// CreateOrganizationRateLimit 创建组织限流规则
func CreateOrganizationRateLimit(c *gin.Context) {
	var req dto.CreateOrganizationRateLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidInput)
		return
	}

	// 验证组织存在
	if req.OrgType == "company" {
		_, err := model.GetCompanyByID(req.OrgId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	} else {
		_, err := model.GetDepartmentByID(req.OrgId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}

	priority := 0
	if req.Priority != nil {
		priority = *req.Priority
	}
	status := 1
	if req.Status != nil {
		status = *req.Status
	}

	// 转换时段
	timeSlots := make(model.TimeSlots, len(req.TimeSlots))
	for i, slot := range req.TimeSlots {
		timeSlots[i] = model.TimeSlot{
			StartTime: slot.StartTime,
			EndTime:   slot.EndTime,
			Weekdays:  slot.Weekdays,
		}
	}

	// 构建规则
	rule := &model.OrganizationRateLimit{
		OrgType:   req.OrgType,
		OrgId:     req.OrgId,
		ModelId:   req.ModelId,
		ModelName: normalizeOptionalString(req.ModelName),
		TimeSlots: timeSlots,
		Rpms:      model.Rpms(req.Rpms),
		Priority:  priority,
		Status:    status,
	}

	// 创建规则
	if err := orgRateLimitService.Create(rule); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"id": rule.Id,
		},
	})
}

// UpdateOrganizationRateLimit 更新组织限流规则
func UpdateOrganizationRateLimit(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	var req dto.UpdateOrganizationRateLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidInput)
		return
	}

	// 获取现有规则
	rule, err := orgRateLimitService.GetByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 转换时段
	timeSlots := make(model.TimeSlots, len(req.TimeSlots))
	for i, slot := range req.TimeSlots {
		timeSlots[i] = model.TimeSlot{
			StartTime: slot.StartTime,
			EndTime:   slot.EndTime,
			Weekdays:  slot.Weekdays,
		}
	}

	// 更新字段
	rule.TimeSlots = timeSlots
	rule.Rpms = model.Rpms(req.Rpms)
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.Status != nil {
		rule.Status = *req.Status
	}

	// 更新规则
	if err := orgRateLimitService.Update(rule); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    nil,
	})
}

// DeleteOrganizationRateLimit 删除组织限流规则
func DeleteOrganizationRateLimit(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	if err := orgRateLimitService.Delete(id); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    nil,
	})
}

// GetOrganizationRateLimits 获取组织限流规则列表
func GetOrganizationRateLimits(c *gin.Context) {
	var req dto.ListOrganizationRateLimitRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidInput)
		return
	}

	rules, err := orgRateLimitService.List(req.OrgType, req.OrgId, req.ModelId, req.Status)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 转换为响应格式
	items := make([]*dto.OrganizationRateLimitData, 0, len(rules))
	for _, rule := range rules {
		item := &dto.OrganizationRateLimitData{
			Id:        rule.Id,
			OrgType:   rule.OrgType,
			OrgId:     rule.OrgId,
			ModelId:   rule.ModelId,
			ModelName: organizationRateLimitModelName(rule),
			TimeSlots: make([]dto.TimeSlot, len(rule.TimeSlots)),
			Rpms:      []int(rule.Rpms),
			Priority:  rule.Priority,
			Status:    rule.Status,
			CreatedAt: rule.CreatedAt,
			UpdatedAt: rule.UpdatedAt,
		}

		// 复制时段
		for i, slot := range rule.TimeSlots {
			item.TimeSlots[i] = dto.TimeSlot{
				StartTime: slot.StartTime,
				EndTime:   slot.EndTime,
				Weekdays:  slot.Weekdays,
			}
		}

		// 获取组织名称
		if rule.OrgType == "company" {
			if company, err := model.GetCompanyByID(rule.OrgId); err == nil {
				item.OrgName = company.Name
			}
		} else {
			if dept, err := model.GetDepartmentByID(rule.OrgId); err == nil {
				item.OrgName = dept.Name
			}
		}

		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items": items,
			"total": len(items),
		},
	})
}

// GetUserEffectiveRateLimit 获取用户当前生效的限流规则
func GetUserEffectiveRateLimit(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	modelName := strings.TrimSpace(c.Query("model_name"))
	if modelIdStr := c.Query("model_id"); modelIdStr != "" {
		if id, err := strconv.ParseInt(modelIdStr, 10, 64); err == nil {
			if modelMeta, err := model.GetModelByID(id); err == nil && modelMeta != nil {
				modelName = modelMeta.ModelName
			}
		}
	}

	// 获取生效规则
	effective, err := orgRateLimitService.GetEffectiveRateLimit(userId, modelName, time.Now())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 如果没有组织规则，检查分组和全局限流
	var data dto.EffectiveRateLimitData
	if effective != nil {
		var timeSlot *dto.TimeSlot
		if effective.TimeSlot != nil {
			timeSlot = &dto.TimeSlot{
				StartTime: effective.TimeSlot.StartTime,
				EndTime:   effective.TimeSlot.EndTime,
				Weekdays:  effective.TimeSlot.Weekdays,
			}
		}

		data = dto.EffectiveRateLimitData{
			Source:    effective.Source,
			OrgName:   effective.OrgName,
			OrgType:   effective.OrgType,
			OrgId:     effective.OrgId,
			ModelId:   effective.ModelId,
			ModelName: effective.ModelName,
			TimeSlot:  timeSlot,
			Rpm:       effective.Rpm,
			Weekday:   int(time.Now().Weekday()),
		}
	} else {
		// 无组织规则，返回分组或全局配置
		data = dto.EffectiveRateLimitData{
			Source:  "none",
			Rpm:     0,
			Weekday: int(time.Now().Weekday()),
		}

		// 这里可以添加获取分组限流的逻辑
		// data.Source = "group"
		// data.Rpm = ...
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    &data,
	})
}

// GetOrganizationRateLimit 获取单个规则详情
func GetOrganizationRateLimit(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	rule, err := orgRateLimitService.GetByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 转换为响应格式
	item := &dto.OrganizationRateLimitData{
		Id:        rule.Id,
		OrgType:   rule.OrgType,
		OrgId:     rule.OrgId,
		ModelId:   rule.ModelId,
		ModelName: organizationRateLimitModelName(rule),
		TimeSlots: make([]dto.TimeSlot, len(rule.TimeSlots)),
		Rpms:      []int(rule.Rpms),
		Priority:  rule.Priority,
		Status:    rule.Status,
		CreatedAt: rule.CreatedAt,
		UpdatedAt: rule.UpdatedAt,
	}

	for i, slot := range rule.TimeSlots {
		item.TimeSlots[i] = dto.TimeSlot{
			StartTime: slot.StartTime,
			EndTime:   slot.EndTime,
			Weekdays:  slot.Weekdays,
		}
	}

	// 获取组织名称
	if rule.OrgType == "company" {
		if company, err := model.GetCompanyByID(rule.OrgId); err == nil {
			item.OrgName = company.Name
		}
	} else {
		if dept, err := model.GetDepartmentByID(rule.OrgId); err == nil {
			item.OrgName = dept.Name
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    item,
	})
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func organizationRateLimitModelName(rule *model.OrganizationRateLimit) string {
	if rule.ModelName != nil {
		return strings.TrimSpace(*rule.ModelName)
	}
	if rule.ModelId != nil {
		if modelMeta, err := model.GetModelByID(*rule.ModelId); err == nil && modelMeta != nil {
			return modelMeta.ModelName
		}
	}
	return ""
}
