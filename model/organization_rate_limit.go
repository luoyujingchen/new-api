package model

import (
	"fmt"
	"strings"
	"gorm.io/gorm"
	"time"
)

// OrganizationRateLimit 组织层级限流规则（公司和部门共用）
// 用于实现公司和部门级别的 RPM（每分钟请求数）限流
type OrganizationRateLimit struct {
	Id          int64          `json:"id" gorm:"primaryKey;autoIncrement"`
	OrgType     string         `json:"org_type" gorm:"type:varchar(10);not null;index:idx_org_type_id,priority:1;index:idx_org_status,priority:1"` // "company" or "department"
	OrgId       int64          `json:"org_id" gorm:"not null;index:idx_org_type_id,priority:2;index:idx_org_status,priority:2"`
	ModelId     *int64         `json:"model_id" gorm:"index:idx_org_model,priority:2"` // nil = all models, references model metadata ID
	ModelName   *string        `json:"model_name,omitempty" gorm:"type:varchar(128);index"` // nil = all models, exact request model name match
	TimeSlots   TimeSlots      `json:"time_slots" gorm:"type:json;serializer:json"` // 时段配置
	Rpms        Rpms           `json:"rpms" gorm:"type:json;serializer:json"`         // 对应时段的 RPM 值
	Priority    int            `json:"priority" gorm:"type:int;default:0;index"`      // 子部门规则更高优先级
	Status      int            `json:"status" gorm:"type:int;default:1;index:idx_org_status,priority:3"` // 1=enabled, 0=disabled
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
	CreatedAt   int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   int64          `json:"updated_at" gorm:"autoUpdateTime"`
}

// TimeSlots 时段配置数组类型
type TimeSlots []TimeSlot

// Rpms RPM 值数组类型
type Rpms []int

// TimeSlot 时段配置
type TimeSlot struct {
	StartTime string `json:"start_time"` // HH:MM 格式，如 "09:00"
	EndTime   string `json:"end_time"`   // HH:MM 格式，如 "18:00"，支持跨天（如 "23:00"-"02:00"）
	Weekdays  []int  `json:"weekdays"`   // 0-6, 0=Sunday, nil 或空数组表示全部
}

// Insert 新建组织限流规则
func (o *OrganizationRateLimit) Insert() error {
	return DB.Create(o).Error
}

// Update 更新组织限流规则
func (o *OrganizationRateLimit) Update() error {
	return DB.Save(o).Error
}

// DeleteByID 根据ID删除组织限流规则
func DeleteOrganizationRateLimitByID(id int64) error {
	return DB.Delete(&OrganizationRateLimit{}, id).Error
}

// GetOrganizationRateLimitByID 根据ID获取组织限流规则
func GetOrganizationRateLimitByID(id int64) (*OrganizationRateLimit, error) {
	var rule OrganizationRateLimit
	err := DB.First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// GetOrganizationRateLimits 获取组织的所有限流规则
func GetOrganizationRateLimits(orgType string, orgId int64, status *int) ([]*OrganizationRateLimit, error) {
	var rules []*OrganizationRateLimit
	query := DB.Where("org_type = ? AND org_id = ?", orgType, orgId)
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	err := query.Order("priority DESC, id ASC").Find(&rules).Error
	return rules, err
}

// GetOrganizationRateLimitByModel 获取组织和模型的限流规则
func GetOrganizationRateLimitByModel(orgType string, orgId int64, modelId *int64, status *int) ([]*OrganizationRateLimit, error) {
	var rules []*OrganizationRateLimit
	query := DB.Where("org_type = ? AND org_id = ?", orgType, orgId)

	if modelId != nil {
		query = query.Where("model_id = ?", *modelId)
	} else {
		query = query.Where("model_id IS NULL")
	}

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	err := query.Order("priority DESC, id ASC").Find(&rules).Error
	return rules, err
}

// MatchTimeSlot 检查给定时间是否匹配时段配置
// 返回匹配的时段索引，如果没有匹配返回 -1
func (ts TimeSlots) MatchTimeSlot(t time.Time) int {
	weekday := int(t.Weekday())
	previousWeekday := (weekday + 6) % 7
	hour := t.Hour()
	minute := t.Minute()
	currentMinutes := hour*60 + minute

	for i, slot := range ts {
		// 解析开始时间
		startHour, startMin, err := parseHHMM(slot.StartTime)
		if err != nil {
			continue
		}
		startMinutes := startHour*60 + startMin

		// 解析结束时间
		endHour, endMin, err := parseHHMM(slot.EndTime)
		if err != nil {
			continue
		}
		endMinutes := endHour*60 + endMin

		// 检查时间是否在时段内
		if startMinutes <= endMinutes {
			// 不跨天时段
			if currentMinutes >= startMinutes && currentMinutes <= endMinutes && slot.matchesWeekday(weekday) {
				return i
			}
		} else {
			// 跨天时段（如 23:00-02:00）
			if currentMinutes >= startMinutes && slot.matchesWeekday(weekday) {
				return i
			}
			if currentMinutes <= endMinutes && slot.matchesWeekday(previousWeekday) {
				return i
			}
		}
	}

	return -1
}

func (slot TimeSlot) matchesWeekday(weekday int) bool {
	if len(slot.Weekdays) == 0 {
		return true
	}
	for _, wd := range slot.Weekdays {
		if wd == weekday {
			return true
		}
	}
	return false
}

// GetRpmForTime 获取给定时间的 RPM 值
func (ts TimeSlots) GetRpmForTime(t time.Time, rpms Rpms) int {
	idx := ts.MatchTimeSlot(t)
	if idx >= 0 && idx < len(rpms) {
		return rpms[idx]
	}
	return 0
}

// parseHHMM 解析 HH:MM 格式的时间
func parseHHMM(s string) (hour, min int, err error) {
	var h, m int
	_, err = fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil {
		return 0, 0, err
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, gorm.ErrInvalidData
	}
	return h, m, nil
}

// Validate 验证组织限流规则
func (o *OrganizationRateLimit) Validate() error {
	// 验证组织类型
	if o.OrgType != "company" && o.OrgType != "department" {
		return gorm.ErrInvalidData
	}
	if o.OrgId <= 0 {
		return gorm.ErrInvalidData
	}
	if o.ModelId != nil && *o.ModelId <= 0 {
		return gorm.ErrInvalidData
	}
	if o.ModelName != nil {
		modelName := strings.TrimSpace(*o.ModelName)
		if modelName == "" {
			return gorm.ErrInvalidData
		}
		o.ModelName = &modelName
	}
	if len(o.TimeSlots) == 0 {
		return gorm.ErrInvalidData
	}

	// 验证时段和 RPM 数量匹配
	if len(o.TimeSlots) != len(o.Rpms) {
		return gorm.ErrInvalidData
	}

	// 验证每个时段
	for _, slot := range o.TimeSlots {
		if slot.StartTime == "" || slot.EndTime == "" {
			return gorm.ErrInvalidData
		}
		// 验证时间格式
		_, _, err := parseHHMM(slot.StartTime)
		if err != nil {
			return err
		}
		_, _, err = parseHHMM(slot.EndTime)
		if err != nil {
			return err
		}
		for _, weekday := range slot.Weekdays {
			if weekday < 0 || weekday > 6 {
				return gorm.ErrInvalidData
			}
		}
	}

	// 验证 RPM 值
	for _, rpm := range o.Rpms {
		if rpm < 0 {
			return gorm.ErrInvalidData
		}
	}

	return nil
}

func BackfillOrganizationRateLimitModelNames() error {
	var rules []OrganizationRateLimit
	if err := DB.Where("model_name IS NULL").Where("model_id IS NOT NULL").Find(&rules).Error; err != nil {
		return err
	}

	for i := range rules {
		if rules[i].ModelId == nil {
			continue
		}
		modelMeta, err := GetModelByID(*rules[i].ModelId)
		if err != nil || modelMeta == nil || modelMeta.ModelName == "" {
			continue
		}
		if err := DB.Model(&OrganizationRateLimit{}).
			Where("id = ?", rules[i].Id).
			Update("model_name", modelMeta.ModelName).Error; err != nil {
			return err
		}
	}

	return nil
}
