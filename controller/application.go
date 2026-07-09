package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var applicationService = service.NewApplicationService()

func buildApplicationResponse(application *model.Application, tokenCount int64) dto.ApplicationResponse {
	if application == nil {
		return dto.ApplicationResponse{}
	}
	return dto.ApplicationResponse{
		Id:                    application.Id,
		AppKey:                application.AppKey,
		Name:                  application.Name,
		Description:           application.Description,
		Status:                application.Status,
		SortOrder:             application.SortOrder,
		HeaderValidationRules: application.GetHeaderValidationRules(),
		HeaderMatchRequired:   application.HeaderMatchRequired,
		CreatedAt:             application.CreatedAt,
		UpdatedAt:             application.UpdatedAt,
		TokenCount:            tokenCount,
	}
}

// GetApplications 获取应用列表
// GET /api/application
func GetApplications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := -1
	if rawStatus, ok := c.GetQuery("status"); ok {
		if parsedStatus, err := strconv.Atoi(rawStatus); err == nil {
			status = parsedStatus
		}
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	result, err := applicationService.ListApplications(page, pageSize, status)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	applicationIds := make([]int64, 0, len(result.Applications))
	for _, application := range result.Applications {
		applicationIds = append(applicationIds, application.Id)
	}
	stats, err := applicationService.GetApplicationStatsBatch(applicationIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 转换为响应格式
	items := make([]dto.ApplicationResponse, len(result.Applications))
	for i, application := range result.Applications {
		app := application
		items[i] = buildApplicationResponse(&app, stats[application.Id])
	}

	common.ApiSuccess(c, gin.H{
		"items":     items,
		"total":     result.Total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetAllApplications 获取所有应用（用于下拉选择）
// GET /api/application/all
func GetAllApplications(c *gin.Context) {
	status := -1
	if rawStatus, ok := c.GetQuery("status"); ok {
		if parsedStatus, err := strconv.Atoi(rawStatus); err == nil {
			status = parsedStatus
		}
	}

	applications, err := applicationService.GetAllApplications(status)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	responses := make([]dto.ApplicationResponse, 0, len(applications))
	for _, application := range applications {
		responses = append(responses, buildApplicationResponse(application, 0))
	}
	common.ApiSuccess(c, responses)
}

// GetSelectableApplications 获取当前用户可选的启用应用。
// GET /api/application/self/all
func GetSelectableApplications(c *gin.Context) {
	applications, err := applicationService.GetAllApplications(1)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	responses := make([]dto.ApplicationResponse, 0, len(applications))
	for _, application := range applications {
		responses = append(responses, buildApplicationResponse(application, 0))
	}
	common.ApiSuccess(c, responses)
}

// GetApplication 获取应用详情
// GET /api/application/:id
func GetApplication(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	application, err := applicationService.GetApplication(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 获取统计信息
	tokenCount, _ := applicationService.GetApplicationStats(id)

	response := buildApplicationResponse(application, tokenCount)

	common.ApiSuccess(c, response)
}

// CreateApplication 创建应用
// POST /api/application
func CreateApplication(c *gin.Context) {
	var req dto.CreateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	status := 1
	if req.Status != nil {
		status = *req.Status
	}
	application, err := applicationService.CreateApplication(
		req.Name,
		req.Description,
		status,
		req.SortOrder,
		req.HeaderValidationRules,
		req.HeaderMatchRequired,
	)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			common.ApiErrorMsg(c, "应用名称已存在")
			return
		}
		common.ApiError(c, err)
		return
	}

	response := buildApplicationResponse(application, 0)

	common.ApiSuccess(c, response)
}

// UpdateApplication 更新应用
// PUT /api/application/:id
func UpdateApplication(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var req dto.UpdateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	err = applicationService.UpdateApplication(
		id,
		req.Name,
		req.Description,
		req.Status,
		req.SortOrder,
		req.HeaderValidationRules,
		req.HeaderMatchRequired,
	)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			common.ApiErrorMsg(c, "应用名称已存在")
			return
		}
		common.ApiError(c, err)
		return
	}

	// 获取更新后的数据
	application, _ := applicationService.GetApplication(id)
	response := buildApplicationResponse(application, 0)

	common.ApiSuccess(c, response)
}

// DeleteApplication 删除应用
// DELETE /api/application/:id
func DeleteApplication(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	err = applicationService.DeleteApplication(id)
	if err != nil {
		if errors.Is(err, gorm.ErrForeignKeyViolated) {
			common.ApiErrorMsg(c, "该应用下存在关联的令牌，无法删除")
			return
		}
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}

// UpdateApplicationStatus 更新应用状态
// PATCH /api/application/:id/status
func UpdateApplicationStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var req struct {
		Status *int `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Status == nil || (*req.Status != 0 && *req.Status != 1) {
		common.ApiErrorMsg(c, "status must be 0 or 1")
		return
	}

	if err := applicationService.UpdateApplicationStatus(id, *req.Status); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}
