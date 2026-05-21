package dto

// CreateOrganizationRateLimitRequest 创建组织限流规则请求
type CreateOrganizationRateLimitRequest struct {
	OrgType   string     `json:"org_type" binding:"required,oneof=company department"`
	OrgId     int64      `json:"org_id" binding:"required,min=1"`
	ModelId   *int64     `json:"model_id"`
	ModelName *string    `json:"model_name"`
	TimeSlots []TimeSlot `json:"time_slots" binding:"required,min=1"`
	Rpms      []int      `json:"rpms" binding:"required,min=1,dive,min=0"`
	Priority  *int       `json:"priority"`
	Status    *int       `json:"status" binding:"omitempty,oneof=0 1"`
}

// UpdateOrganizationRateLimitRequest 更新组织限流规则请求
type UpdateOrganizationRateLimitRequest struct {
	TimeSlots []TimeSlot `json:"time_slots" binding:"required,min=1"`
	Rpms      []int      `json:"rpms" binding:"required,min=1,dive,min=0"`
	Priority  *int       `json:"priority"`
	Status    *int       `json:"status" binding:"omitempty,oneof=0 1"`
}

// TimeSlot 时段配置
type TimeSlot struct {
	StartTime string `json:"start_time" binding:"required"` // HH:MM 格式
	EndTime   string `json:"end_time" binding:"required"`   // HH:MM 格式
	Weekdays  []int  `json:"weekdays"`                       // 0-6, 0=Sunday
}

// ListOrganizationRateLimitRequest 查询组织限流规则请求
type ListOrganizationRateLimitRequest struct {
	OrgType   string  `form:"org_type" binding:"required,oneof=company department"`
	OrgId     int64   `form:"org_id" binding:"required,min=1"`
	ModelId   *int64  `form:"model_id"`
	ModelName *string `form:"model_name"`
	Status    *int    `form:"status" binding:"omitempty,oneof=0 1"`
}

// EffectiveRateLimitResponse 有效限流规则响应
type EffectiveRateLimitResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message,omitempty"`
	Data    *EffectiveRateLimitData `json:"data,omitempty"`
}

// EffectiveRateLimitData 有效限流规则数据
type EffectiveRateLimitData struct {
	Source    string    `json:"source"` // "department", "company", "group", "global", "none"
	OrgName   string    `json:"org_name,omitempty"`
	OrgType   string    `json:"org_type,omitempty"`
	OrgId     int64     `json:"org_id,omitempty"`
	ModelId   *int64    `json:"model_id"`
	ModelName string    `json:"model_name,omitempty"`
	TimeSlot  *TimeSlot `json:"time_slot,omitempty"`
	Rpm       int       `json:"rpm"`
	Weekday   int       `json:"weekday,omitempty"` // 当前星期几
}

// OrganizationRateLimitResponse 组织限流规则响应
type OrganizationRateLimitResponse struct {
	Success bool                       `json:"success"`
	Message string                     `json:"message,omitempty"`
	Data    *OrganizationRateLimitData `json:"data,omitempty"`
}

// OrganizationRateLimitData 组织限流规则数据
type OrganizationRateLimitData struct {
	Id          int64       `json:"id"`
	OrgType     string      `json:"org_type"`
	OrgId       int64       `json:"org_id"`
	OrgName     string      `json:"org_name,omitempty"`
	ModelId     *int64      `json:"model_id"`
	ModelName   string      `json:"model_name,omitempty"`
	TimeSlots   []TimeSlot  `json:"time_slots"`
	Rpms        []int       `json:"rpms"`
	Priority    int         `json:"priority"`
	Status      int         `json:"status"`
	CreatedAt   int64       `json:"created_at"`
	UpdatedAt   int64       `json:"updated_at"`
}

// OrganizationRateLimitListResponse 组织限流规则列表响应
type OrganizationRateLimitListResponse struct {
	Success bool                           `json:"success"`
	Message string                         `json:"message,omitempty"`
	Data    *OrganizationRateLimitListData `json:"data,omitempty"`
}

// OrganizationRateLimitListData 组织限流规则列表数据
type OrganizationRateLimitListData struct {
	Items []*OrganizationRateLimitData `json:"items"`
	Total int                         `json:"total"`
}
