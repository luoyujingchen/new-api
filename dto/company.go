package dto

// CreateCompanyRequest 创建公司请求
type CreateCompanyRequest struct {
	Name        string `json:"name" binding:"required,max=128"`
	Code        string `json:"code" binding:"required,max=32"`
	Description string `json:"description" binding:"max=500"`
	Status      int    `json:"status" binding:"oneof=0 1"`
	SortOrder   int    `json:"sort_order"`
	QueuePriority int  `json:"queue_priority"`
}

// UpdateCompanyRequest 更新公司请求
type UpdateCompanyRequest struct {
	Name        string `json:"name" binding:"required,max=128"`
	Code        string `json:"code" binding:"required,max=32"`
	Description string `json:"description" binding:"max=500"`
	Status      int    `json:"status" binding:"oneof=0 1"`
	SortOrder   int    `json:"sort_order"`
	QueuePriority int  `json:"queue_priority"`
}

// CompanyResponse 公司响应
type CompanyResponse struct {
	Id             int64  `json:"id"`
	Name           string `json:"name"`
	Code           string `json:"code"`
	Description    string `json:"description"`
	Status         int    `json:"status"`
	SortOrder      int    `json:"sort_order"`
	QueuePriority  int    `json:"queue_priority"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
	DepartmentCount int64 `json:"department_count,omitempty"`
	UserCount      int64  `json:"user_count,omitempty"`
}

// CompanyListResponse 公司列表响应
type CompanyListResponse struct {
	Items []CompanyResponse `json:"items"`
	Total int64             `json:"total"`
}
