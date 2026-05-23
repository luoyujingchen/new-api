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

// GetDepartments returns paginated department list
func GetDepartments(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	companyId := 0
	status := 0

	if cid := c.Query("company_id"); cid != "" {
		if v, err := strconv.Atoi(cid); err == nil {
			companyId = v
		}
	}
	if statusStr := c.Query("status"); statusStr != "" {
		if s, err := strconv.Atoi(statusStr); err == nil {
			status = s
		}
	}

	departments, total, err := model.GetDepartments(pageInfo, companyId, status)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// Build response with counts and parent/company info
	items := make([]*dto.DepartmentResponse, 0, len(departments))
	for _, dept := range departments {
		resp, _ := service.GetDepartmentResponse(dept)
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

// GetAllDepartments returns all departments (no pagination)
func GetAllDepartments(c *gin.Context) {
	companyId := 0
	status := 0

	if cid := c.Query("company_id"); cid != "" {
		if v, err := strconv.Atoi(cid); err == nil {
			companyId = v
		}
	}
	if statusStr := c.Query("status"); statusStr != "" {
		if s, err := strconv.Atoi(statusStr); err == nil {
			status = s
		}
	}

	departments, err := model.GetAllDepartments(companyId, status)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    departments,
	})
}

// GetDepartmentTree returns departments as a flat list for a specific company
func GetDepartmentTree(c *gin.Context) {
	companyIdStr := c.Query("company_id")
	if companyIdStr == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	companyId, err := strconv.Atoi(companyIdStr)
	if err != nil || companyId == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	departments, err := model.GetDepartmentTree(companyId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	// Build response with counts
	items := make([]*dto.DepartmentResponse, 0, len(departments))
	for _, dept := range departments {
		resp, _ := service.GetDepartmentResponse(dept)
		if resp != nil {
			items = append(items, resp)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    items,
	})
}

// GetDepartment returns a single department by ID
func GetDepartment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	dept, err := model.GetDepartmentById(id)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgNotFound)
		return
	}

	resp, err := service.GetDepartmentResponse(dept)
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

// CreateDepartment creates a new department
func CreateDepartment(c *gin.Context) {
	var req dto.DepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	dept, err := service.CreateDepartmentService(&req)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	resp, _ := service.GetDepartmentResponse(dept)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    resp,
	})
}

// UpdateDepartment updates an existing department
func UpdateDepartment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	var req dto.DepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	dept, err := service.UpdateDepartmentService(id, &req)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	resp, _ := service.GetDepartmentResponse(dept)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    resp,
	})
}

// DeleteDepartment deletes a department
func DeleteDepartment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	if err := service.DeleteDepartmentService(id); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// MoveDepartment moves a department to a new parent
func MoveDepartment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	var req dto.DepartmentMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	dept, err := service.MoveDepartmentService(id, &req)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	resp, _ := service.GetDepartmentResponse(dept)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    resp,
	})
}

// UpdateDepartmentStatus enables or disables a department
func UpdateDepartmentStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	var req dto.DepartmentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	if err := service.UpdateDepartmentStatusService(id, req.Status); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// GetDepartmentUsers returns users belonging to a department
func GetDepartmentUsers(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	pageInfo := common.GetPageQuery(c)
	users, total, err := service.GetDepartmentUsersService(id, pageInfo)
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

// SetUserDepartment sets or clears a user's department affiliation
func SetUserDepartment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	var req dto.UserDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	if err := service.SetUserDepartmentService(id, &req); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// ClearUserDepartment clears a user's department affiliation
func ClearUserDepartment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	if err := model.ClearUserDepartment(id); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
