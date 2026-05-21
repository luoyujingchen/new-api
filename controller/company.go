package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var companyService = service.NewCompanyService()

// GetCompanies 获取公司列表
// GET /api/company
func GetCompanies(c *gin.Context) {
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

	result, err := companyService.ListCompanies(page, pageSize, status)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 转换为响应格式
	items := make([]dto.CompanyResponse, len(result.Companies))
	for i, company := range result.Companies {
		deptCount, userCount, err := companyService.GetCompanyStats(company.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}

		items[i] = dto.CompanyResponse{
			Id:              company.Id,
			Name:            company.Name,
			Code:            company.Code,
			Description:     company.Description,
			Status:          company.Status,
			SortOrder:       company.SortOrder,
			CreatedAt:       company.CreatedAt,
			UpdatedAt:       company.UpdatedAt,
			DepartmentCount: deptCount,
			UserCount:       userCount,
		}
	}

	common.ApiSuccess(c, gin.H{
		"items": items,
		"total": result.Total,
		"page":  page,
		"page_size": pageSize,
	})
}

// GetAllCompanies 获取所有公司（用于下拉选择）
// GET /api/company/all
func GetAllCompanies(c *gin.Context) {
	status := -1
	if rawStatus, ok := c.GetQuery("status"); ok {
		if parsedStatus, err := strconv.Atoi(rawStatus); err == nil {
			status = parsedStatus
		}
	}

	companies, err := companyService.GetAllCompanies(status)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, companies)
}

// GetCompany 获取公司详情
// GET /api/company/:id
func GetCompany(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	company, err := companyService.GetCompany(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 获取统计信息
	deptCount, userCount, _ := companyService.GetCompanyStats(id)

	response := dto.CompanyResponse{
		Id:             company.Id,
		Name:           company.Name,
		Code:           company.Code,
		Description:    company.Description,
		Status:         company.Status,
		SortOrder:      company.SortOrder,
		CreatedAt:      company.CreatedAt,
		UpdatedAt:      company.UpdatedAt,
		DepartmentCount: deptCount,
		UserCount:      userCount,
	}

	common.ApiSuccess(c, response)
}

// CreateCompany 创建公司
// POST /api/company
func CreateCompany(c *gin.Context) {
	var req dto.CreateCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	company, err := companyService.CreateCompany(
		req.Name,
		req.Code,
		req.Description,
		req.Status,
		req.SortOrder,
	)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			common.ApiErrorMsg(c, "公司名称或代码已存在")
			return
		}
		common.ApiError(c, err)
		return
	}

	response := dto.CompanyResponse{
		Id:          company.Id,
		Name:        company.Name,
		Code:        company.Code,
		Description: company.Description,
		Status:      company.Status,
		SortOrder:   company.SortOrder,
		CreatedAt:   company.CreatedAt,
		UpdatedAt:   company.UpdatedAt,
	}

	common.ApiSuccess(c, response)
}

// UpdateCompany 更新公司
// PUT /api/company/:id
func UpdateCompany(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var req dto.UpdateCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	err = companyService.UpdateCompany(
		id,
		req.Name,
		req.Code,
		req.Description,
		req.Status,
		req.SortOrder,
	)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			common.ApiErrorMsg(c, "公司名称或代码已存在")
			return
		}
		common.ApiError(c, err)
		return
	}

	// 获取更新后的数据
	company, _ := companyService.GetCompany(id)
	response := dto.CompanyResponse{
		Id:          company.Id,
		Name:        company.Name,
		Code:        company.Code,
		Description: company.Description,
		Status:      company.Status,
		SortOrder:   company.SortOrder,
		CreatedAt:   company.CreatedAt,
		UpdatedAt:   company.UpdatedAt,
	}

	common.ApiSuccess(c, response)
}

// DeleteCompany 删除公司
// DELETE /api/company/:id
func DeleteCompany(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	err = companyService.DeleteCompany(id)
	if err != nil {
		if errors.Is(err, gorm.ErrForeignKeyViolated) {
			common.ApiErrorMsg(c, "该公司下存在部门或用户，无法删除")
			return
		}
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}

// UpdateCompanyStatus 更新公司状态
// PATCH /api/company/:id/status
func UpdateCompanyStatus(c *gin.Context) {
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

	if err := companyService.UpdateCompanyStatus(id, *req.Status); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}
