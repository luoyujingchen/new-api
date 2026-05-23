package dto

// Company request/response DTOs

type CompanyRequest struct {
	Name        string `json:"name" binding:"required,max=128"`
	Code        string `json:"code" binding:"required,max=64"`
	Description string `json:"description" binding:"max=512"`
	Status      *int   `json:"status,omitempty"`
	SortOrder   *int   `json:"sort_order,omitempty"`
}

type CompanyStatusRequest struct {
	Status int `json:"status" binding:"required,oneof=1 2"`
}

type CompanyResponse struct {
	Id              int    `json:"id"`
	Name            string `json:"name"`
	Code            string `json:"code"`
	Description     string `json:"description"`
	Status          int    `json:"status"`
	SortOrder       int    `json:"sort_order"`
	DepartmentCount int64  `json:"department_count"`
	UserCount       int64  `json:"user_count"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

// Department request/response DTOs

type DepartmentRequest struct {
	CompanyId   int    `json:"company_id" binding:"required"`
	Name        string `json:"name" binding:"required,max=128"`
	ParentId    *int   `json:"parent_id,omitempty"`
	Description string `json:"description" binding:"max=512"`
	Status      *int   `json:"status,omitempty"`
	SortOrder   *int   `json:"sort_order,omitempty"`
}

type DepartmentMoveRequest struct {
	TargetParentId int `json:"target_parent_id"`
}

type DepartmentStatusRequest struct {
	Status int `json:"status" binding:"required,oneof=1 2"`
}

type DepartmentResponse struct {
	Id          int    `json:"id"`
	CompanyId   int    `json:"company_id"`
	ParentId    int    `json:"parent_id"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Status      int    `json:"status"`
	SortOrder   int    `json:"sort_order"`
	ChildCount  int64  `json:"child_count"`
	UserCount   int64  `json:"user_count"`
	CompanyName string `json:"company_name,omitempty"`
	ParentName  string `json:"parent_name,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// UserDepartment request DTOs

type UserDepartmentRequest struct {
	CompanyId    *int `json:"company_id,omitempty"`
	DepartmentId *int `json:"department_id,omitempty"`
}
