package dto

// TimeSlot represents a time range with optional weekday filter
type TimeSlot struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Weekdays  []int  `json:"weekdays,omitempty"`
}

// CreateRateLimitRequest is the request body for creating a rate limit rule
type CreateRateLimitRequest struct {
	OrgType   string     `json:"org_type" binding:"required,oneof=company department"`
	OrgId     int        `json:"org_id" binding:"required,min=1"`
	ModelId   int        `json:"model_id,omitempty"`
	ModelName string     `json:"model_name,omitempty"`
	TimeSlots []TimeSlot `json:"time_slots" binding:"required,min=1"`
	Rpms      []int      `json:"rpms" binding:"required,min=1"`
	Priority  int        `json:"priority"`
	Status    int        `json:"status" binding:"required,oneof=1 2"`
}

// UpdateRateLimitRequest is the request body for updating a rate limit rule
type UpdateRateLimitRequest struct {
	TimeSlots []TimeSlot `json:"time_slots" binding:"required,min=1"`
	Rpms      []int      `json:"rpms" binding:"required,min=1"`
	Priority  int        `json:"priority"`
	Status    int        `json:"status" binding:"required,oneof=1 2"`
}

// RateLimitResponse is the response for a single rate limit rule
type RateLimitResponse struct {
	Id         int        `json:"id"`
	OrgType    string     `json:"org_type"`
	OrgId      int        `json:"org_id"`
	OrgName    string     `json:"org_name,omitempty"`
	ModelId    int        `json:"model_id"`
	ModelName  string     `json:"model_name"`
	TimeSlots  []TimeSlot `json:"time_slots"`
	Rpms       []int      `json:"rpms"`
	Priority   int        `json:"priority"`
	Status     int        `json:"status"`
	CreatedAt  int64      `json:"created_at"`
	UpdatedAt  int64      `json:"updated_at"`
}

// RateLimitListQuery is the query parameters for listing rate limit rules
type RateLimitListQuery struct {
	OrgType string `form:"org_type" binding:"required,oneof=company department"`
	OrgId   int    `form:"org_id" binding:"required,min=1"`
	ModelId int    `form:"model_id,omitempty"`
	Status  int    `form:"status,omitempty"`
}

// UserRateLimitQuery is the query parameters for querying a user's effective rate limit
type UserRateLimitQuery struct {
	ModelName string `form:"model_name,omitempty"`
	ModelId   int    `form:"model_id,omitempty"`
}

// UserRateLimitResponse is the response for a user's effective rate limit
type UserRateLimitResponse struct {
	Source    string    `json:"source"`    // department, company, none
	OrgName   string    `json:"org_name,omitempty"`
	OrgType   string    `json:"org_type,omitempty"`
	OrgId     int       `json:"org_id,omitempty"`
	TimeSlot  *TimeSlot `json:"time_slot,omitempty"`
	Rpm       int       `json:"rpm,omitempty"`
	Weekday   int       `json:"weekday,omitempty"`
	ModelId   int       `json:"model_id,omitempty"`
	ModelName string    `json:"model_name,omitempty"`
	Priority  int       `json:"priority,omitempty"`
	RuleId    int       `json:"rule_id,omitempty"`
}
