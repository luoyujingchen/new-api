package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// GetCompanies returns paginated company list
func GetCompanies(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status := 0
	if statusStr := c.Query("status"); statusStr != "" {
		if s, err := strconv.Atoi(statusStr); err == nil {
			status = s
		}
	}

	companies, total, err := model.GetCompanies(pageInfo, status)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// Build response with counts
	items := make([]*dto.CompanyResponse, 0, len(companies))
	for _, comp := range companies {
		resp, _ := service.GetCompanyResponse(comp)
		if resp != nil {
			items = append(items, resp)
		}
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    pageInfo,
	})
}

// GetAllCompanies returns all companies (no pagination)
func GetAllCompanies(c *gin.Context) {
	status := 0
	if statusStr := c.Query("status"); statusStr != "" {
		if s, err := strconv.Atoi(statusStr); err == nil {
			status = s
		}
	}

	companies, err := model.GetAllCompanies(status)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    companies,
	})
}

// GetCompany returns a single company by ID
func GetCompany(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	company, err := model.GetCompanyById(id)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgNotFound)
		return
	}

	resp, err := service.GetCompanyResponse(company)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    resp,
	})
}

// CreateCompany creates a new company
func CreateCompany(c *gin.Context) {
	var req dto.CompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	company, err := service.CreateCompany(&req)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	resp, _ := service.GetCompanyResponse(company)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    resp,
	})
}

// UpdateCompany updates an existing company
func UpdateCompany(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	var req dto.CompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	company, err := service.UpdateCompanyService(id, &req)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	resp, _ := service.GetCompanyResponse(company)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    resp,
	})
}

// DeleteCompany deletes a company
func DeleteCompany(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	if err := service.DeleteCompanyService(id); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// UpdateCompanyStatus enables or disables a company
func UpdateCompanyStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	var req dto.CompanyStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	if err := service.UpdateCompanyStatusService(id, req.Status); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// GetCompanyUsers returns users belonging to a company
func GetCompanyUsers(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	pageInfo := common.GetPageQuery(c)
	users, total, err := service.GetCompanyUsersService(id, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    pageInfo,
	})
}
