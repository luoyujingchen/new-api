package dto

// CreateApplicationRequest 创建应用请求
type CreateApplicationRequest struct {
	Name        string `json:"name" binding:"required,max=128"`
	Description string `json:"description" binding:"max=500"`
	Status      int    `json:"status" binding:"oneof=0 1"`
	SortOrder   int    `json:"sort_order"`
}

// UpdateApplicationRequest 更新应用请求
type UpdateApplicationRequest struct {
	Name        string `json:"name" binding:"required,max=128"`
	Description string `json:"description" binding:"max=500"`
	Status      int    `json:"status" binding:"oneof=0 1"`
	SortOrder   int    `json:"sort_order"`
}

// ApplicationResponse 应用响应
type ApplicationResponse struct {
	Id          int64  `json:"id"`
	AppKey      string `json:"app_key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      int    `json:"status"`
	SortOrder   int    `json:"sort_order"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
	TokenCount  int64  `json:"token_count,omitempty"`
}
