package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var departmentService = service.NewDepartmentService()
var userDeptService = service.NewUserDepartmentService()

// GetDepartments 获取部门列表
// GET /api/department
func GetDepartments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	companyId, _ := strconv.ParseInt(c.Query("company_id"), 10, 64)
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

	result, err := departmentService.ListDepartments(page, pageSize, companyId, status)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 转换为响应格式
	items := make([]dto.DepartmentResponse, len(result.Departments))
	for i, dept := range result.Departments {
		childCount, userCount, err := departmentService.GetDepartmentStats(dept.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}

		items[i] = dto.DepartmentResponse{
			Id:          dept.Id,
			CompanyId:   dept.CompanyId,
			Name:        dept.Name,
			ParentId:    dept.ParentId,
			Level:       dept.Level,
			Path:        dept.Path,
			Description: dept.Description,
			Status:      dept.Status,
			SortOrder:   dept.SortOrder,
			CreatedAt:   dept.CreatedAt,
			UpdatedAt:   dept.UpdatedAt,
			ChildCount:  childCount,
			UserCount:   userCount,
		}
		// 添加公司信息
		if dept.Company.Id != 0 {
			items[i].Company = &dto.CompanySummary{
				Id:   dept.Company.Id,
				Name: dept.Company.Name,
				Code: dept.Company.Code,
			}
		}
		// 添加父部门信息
		if dept.Parent != nil {
			items[i].Parent = &dto.DepartmentSummary{
				Id:   dept.Parent.Id,
				Name: dept.Parent.Name,
				Path: dept.Parent.Path,
			}
		}
	}

	common.ApiSuccess(c, gin.H{
		"items": items,
		"total": result.Total,
		"page":  page,
		"page_size": pageSize,
	})
}

// GetAllDepartments 获取所有部门（用于下拉选择）
// GET /api/department/all
func GetAllDepartments(c *gin.Context) {
	companyId, _ := strconv.ParseInt(c.Query("company_id"), 10, 64)
	status := -1
	if rawStatus, ok := c.GetQuery("status"); ok {
		if parsedStatus, err := strconv.Atoi(rawStatus); err == nil {
			status = parsedStatus
		}
	}

	departments, err := departmentService.GetAllDepartments(companyId, status)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, departments)
}

// GetDepartmentTree 获取部门树
// GET /api/department/tree
func GetDepartmentTree(c *gin.Context) {
	companyId, _ := strconv.ParseInt(c.Query("company_id"), 10, 64)
	if companyId == 0 {
		common.ApiErrorMsg(c, "company_id 参数必填")
		return
	}

	tree, err := departmentService.GetDepartmentTree(companyId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 转换为响应格式
	response := make([]*dto.DepartmentTreeNodeResponse, len(tree))
	for i, node := range tree {
		response[i] = convertDeptTreeNode(node)
	}

	common.ApiSuccess(c, response)
}

// convertDeptTreeNode 递归转换部门树节点
func convertDeptTreeNode(node *model.DepartmentTreeNode) *dto.DepartmentTreeNodeResponse {
	response := &dto.DepartmentTreeNodeResponse{
		DepartmentResponse: dto.DepartmentResponse{
			Id:          node.Id,
			CompanyId:   node.CompanyId,
			Name:        node.Name,
			ParentId:    node.ParentId,
			Level:       node.Level,
			Path:        node.Path,
			Description: node.Description,
			Status:      node.Status,
			SortOrder:   node.SortOrder,
			CreatedAt:   node.CreatedAt,
			UpdatedAt:   node.UpdatedAt,
		},
	}

	if len(node.Children) > 0 {
		response.Children = make([]*dto.DepartmentTreeNodeResponse, len(node.Children))
		for i, child := range node.Children {
			response.Children[i] = convertDeptTreeNode(child)
		}
	}

	return response
}

// GetDepartment 获取部门详情
// GET /api/department/:id
func GetDepartment(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	dept, err := departmentService.GetDepartment(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 获取统计信息
	childCount, userCount, _ := departmentService.GetDepartmentStats(id)

	response := dto.DepartmentResponse{
		Id:          dept.Id,
		CompanyId:   dept.CompanyId,
		Name:        dept.Name,
		ParentId:    dept.ParentId,
		Level:       dept.Level,
		Path:        dept.Path,
		Description: dept.Description,
		Status:      dept.Status,
		SortOrder:   dept.SortOrder,
		CreatedAt:   dept.CreatedAt,
		UpdatedAt:   dept.UpdatedAt,
		ChildCount:  childCount,
		UserCount:   userCount,
	}

	// 添加公司信息
	if dept.Company.Id != 0 {
		response.Company = &dto.CompanySummary{
			Id:   dept.Company.Id,
			Name: dept.Company.Name,
			Code: dept.Company.Code,
		}
	}

	// 添加父部门信息
	if dept.Parent != nil {
		response.Parent = &dto.DepartmentSummary{
			Id:   dept.Parent.Id,
			Name: dept.Parent.Name,
			Path: dept.Parent.Path,
		}
	}

	common.ApiSuccess(c, response)
}

// CreateDepartment 创建部门
// POST /api/department
func CreateDepartment(c *gin.Context) {
	var req dto.CreateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	dept, err := departmentService.CreateDepartment(
		req.CompanyId,
		req.Name,
		req.ParentId,
		req.Description,
		req.Status,
		req.SortOrder,
	)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			common.ApiErrorMsg(c, "同一父部门下已存在同名部门")
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "公司或父部门不存在")
			return
		}
		if errors.Is(err, gorm.ErrInvalidData) || strings.Contains(err.Error(), "department hierarchy depth cannot exceed 4 levels") {
			common.ApiErrorMsg(c, "部门层级深度不能超过4级")
			return
		}
		common.ApiError(c, err)
		return
	}

	response := dto.DepartmentResponse{
		Id:          dept.Id,
		CompanyId:   dept.CompanyId,
		Name:        dept.Name,
		ParentId:    dept.ParentId,
		Level:       dept.Level,
		Path:        dept.Path,
		Description: dept.Description,
		Status:      dept.Status,
		SortOrder:   dept.SortOrder,
		CreatedAt:   dept.CreatedAt,
		UpdatedAt:   dept.UpdatedAt,
	}

	common.ApiSuccess(c, response)
}

// UpdateDepartment 更新部门
// PUT /api/department/:id
func UpdateDepartment(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var req dto.UpdateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	err = departmentService.UpdateDepartment(
		id,
		req.Name,
		req.Description,
		req.Status,
		req.SortOrder,
	)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			common.ApiErrorMsg(c, "同一父部门下已存在同名部门")
			return
		}
		common.ApiError(c, err)
		return
	}

	// 获取更新后的数据
	dept, _ := departmentService.GetDepartment(id)
	response := dto.DepartmentResponse{
		Id:          dept.Id,
		CompanyId:   dept.CompanyId,
		Name:        dept.Name,
		ParentId:    dept.ParentId,
		Level:       dept.Level,
		Path:        dept.Path,
		Description: dept.Description,
		Status:      dept.Status,
		SortOrder:   dept.SortOrder,
		CreatedAt:   dept.CreatedAt,
		UpdatedAt:   dept.UpdatedAt,
	}

	common.ApiSuccess(c, response)
}

// DeleteDepartment 删除部门
// DELETE /api/department/:id
func DeleteDepartment(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	err = departmentService.DeleteDepartment(id)
	if err != nil {
		if errors.Is(err, gorm.ErrForeignKeyViolated) {
			common.ApiErrorMsg(c, "该部门下存在子部门或用户，无法删除")
			return
		}
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}

// MoveDepartment 移动部门
// POST /api/department/:id/move
func MoveDepartment(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var req dto.MoveDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	// 获取当前部门以获取公司ID
	currentDept, err := departmentService.GetDepartment(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	err = departmentService.MoveDepartment(id, req.ParentId, currentDept.CompanyId)
	if err != nil {
		if err.Error() == "cannot move department to its descendant" {
			common.ApiErrorMsg(c, "不能将部门移动到其子孙部门下")
			return
		}
		if err.Error() == "cannot move department to itself" {
			common.ApiErrorMsg(c, "不能将部门移动到自己下面")
			return
		}
		if err.Error() == "parent department must be in the same company" {
			common.ApiErrorMsg(c, "父部门必须属于同一公司")
			return
		}
		if err.Error() == "department hierarchy depth cannot exceed 4 levels" {
			common.ApiErrorMsg(c, "移动后部门层级深度将超过4级")
			return
		}
		if err.Error() == "record not found" {
			common.ApiErrorMsg(c, "父部门不存在")
			return
		}
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}

// GetDepartmentUsers 获取部门下的用户列表
// GET /api/department/:id/users
func GetDepartmentUsers(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	users, total, err := departmentService.GetDepartmentUsers(id, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"items": users,
		"total": total,
		"page":  page,
		"page_size": pageSize,
	})
}

// UpdateDepartmentStatus 更新部门状态
// PATCH /api/department/:id/status
func UpdateDepartmentStatus(c *gin.Context) {
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

	if err := departmentService.UpdateDepartmentStatus(id, *req.Status); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}

// SetUserDepartment 设置用户的部门和公司
// PUT /api/user/:id/department
func SetUserDepartment(c *gin.Context) {
	userIdStr := c.Param("id")
	userId, err := strconv.Atoi(userIdStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var req dto.SetUserDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	err = userDeptService.SetUserDepartment(userId, req.CompanyId, req.DepartmentId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "公司或部门不存在")
			return
		}
		if errors.Is(err, gorm.ErrInvalidData) {
			common.ApiErrorMsg(c, "部门不属于指定公司")
			return
		}
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}

// ClearUserDepartment 清空用户的部门和公司
// DELETE /api/user/:id/department
func ClearUserDepartment(c *gin.Context) {
	userIdStr := c.Param("id")
	userId, err := strconv.Atoi(userIdStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if err := userDeptService.ClearUserDepartment(userId); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}

// GetCompanyUsers 获取公司下的用户列表
// GET /api/company/:id/users
func GetCompanyUsers(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	users, total, err := departmentService.GetCompanyUsers(id, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"items": users,
		"total": total,
		"page":  page,
		"page_size": pageSize,
	})
}
