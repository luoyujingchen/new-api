package dto

// CreateDepartmentRequest 创建部门请求
type CreateDepartmentRequest struct {
	CompanyId   int64  `json:"company_id" binding:"required"`
	Name        string `json:"name" binding:"required,max=128"`
	ParentId    *int64 `json:"parent_id"`
	Description string `json:"description" binding:"max=500"`
	Status      int    `json:"status" binding:"oneof=0 1"`
	SortOrder   int    `json:"sort_order"`
}

// UpdateDepartmentRequest 更新部门请求
type UpdateDepartmentRequest struct {
	Name        string `json:"name" binding:"required,max=128"`
	Description string `json:"description" binding:"max=500"`
	Status      int    `json:"status" binding:"oneof=0 1"`
	SortOrder   int    `json:"sort_order"`
}

// MoveDepartmentRequest 移动部门请求
type MoveDepartmentRequest struct {
	ParentId *int64 `json:"parent_id"` // 新的父部门ID，null表示移动到根级别
}

// DepartmentResponse 部门响应
type DepartmentResponse struct {
	Id          int64              `json:"id"`
	CompanyId   int64              `json:"company_id"`
	Name        string             `json:"name"`
	ParentId    *int64             `json:"parent_id"`
	Level       int                `json:"level"`
	Path        string             `json:"path"`
	Description string             `json:"description"`
	Status      int                `json:"status"`
	SortOrder   int                `json:"sort_order"`
	CreatedAt   int64              `json:"created_at"`
	UpdatedAt   int64              `json:"updated_at"`
	Company     *CompanySummary    `json:"company,omitempty"`
	Parent      *DepartmentSummary `json:"parent,omitempty"`
	ChildCount  int64              `json:"child_count,omitempty"`
	UserCount   int64              `json:"user_count,omitempty"`
}

// DepartmentTreeNodeResponse 部门树节点响应
type DepartmentTreeNodeResponse struct {
	DepartmentResponse
	Children []*DepartmentTreeNodeResponse `json:"children,omitempty"`
}

// DepartmentSummary 部门摘要（用于嵌套显示）
type DepartmentSummary struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// CompanySummary 公司摘要（用于嵌套显示）
type CompanySummary struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

// DepartmentListResponse 部门列表响应
type DepartmentListResponse struct {
	Items []DepartmentResponse `json:"items"`
	Total int64                `json:"total"`
}

// SetUserDepartmentRequest 设置用户部门请求
type SetUserDepartmentRequest struct {
	CompanyId    *int64 `json:"company_id"`
	DepartmentId *int64 `json:"department_id"`
}

// UserDepartmentResponse 用户部门响应
type UserDepartmentResponse struct {
	UserId       int               `json:"user_id"`
	Username     string            `json:"username"`
	DisplayName  string            `json:"display_name"`
	CompanyId    *int64            `json:"company_id,omitempty"`
	DepartmentId *int64            `json:"department_id,omitempty"`
	Company      *CompanySummary   `json:"company,omitempty"`
	Department   *DepartmentSummary `json:"department,omitempty"`
}
