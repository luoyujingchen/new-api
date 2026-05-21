package service

import (
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// DepartmentService 部门服务
type DepartmentService struct{}

// NewDepartmentService 创建部门服务实例
func NewDepartmentService() *DepartmentService {
	return &DepartmentService{}
}

// CreateDepartment 创建部门
func (s *DepartmentService) CreateDepartment(companyId int64, name string, parentId *int64, description string, status int, sortOrder int) (*model.Department, error) {
	// 验证公司存在
	_, err := model.GetCompanyByID(companyId)
	if err != nil {
		return nil, gorm.ErrRecordNotFound
	}

	// 如果指定了父部门，验证父部门存在且属于同一公司
	if parentId != nil && *parentId > 0 {
		parent, err := model.GetDepartmentByID(*parentId)
		if err != nil {
			return nil, gorm.ErrRecordNotFound
		}
		if parent.CompanyId != companyId {
			return nil, gorm.ErrInvalidData
		}
		// 检查层级深度
		if parent.Level >= 4 {
			return nil, gorm.ErrInvalidData
		}
	}

	// 检查同一公司下同一父部门的名称是否重复
	duplicated, err := model.IsDepartmentNameDuplicated(0, companyId, name, parentId)
	if err != nil {
		return nil, err
	}
	if duplicated {
		return nil, gorm.ErrDuplicatedKey
	}

	dept := &model.Department{
		CompanyId:   companyId,
		Name:        name,
		ParentId:    parentId,
		Description: description,
		Status:      status,
		SortOrder:   sortOrder,
	}

	if err := dept.Insert(); err != nil {
		return nil, err
	}

	// 重新加载带关联的数据
	return model.GetDepartmentByID(dept.Id)
}

// UpdateDepartment 更新部门
func (s *DepartmentService) UpdateDepartment(id int64, name string, description string, status int, sortOrder int) error {
	dept, err := model.GetDepartmentByID(id)
	if err != nil {
		return err
	}

	// 检查名称是否重复（排除自己）
	duplicated, err := model.IsDepartmentNameDuplicated(id, dept.CompanyId, name, dept.ParentId)
	if err != nil {
		return err
	}
	if duplicated {
		return gorm.ErrDuplicatedKey
	}

	dept.Name = name
	dept.Description = description
	dept.Status = status
	dept.SortOrder = sortOrder

	return dept.Update()
}

// DeleteDepartment 删除部门
func (s *DepartmentService) DeleteDepartment(id int64) error {
	// 检查是否有子部门
	hasChildren, err := model.HasChildren(id)
	if err != nil {
		return err
	}
	if hasChildren {
		return gorm.ErrForeignKeyViolated
	}

	// 检查是否有关联的用户
	hasUsers, err := model.HasUsersInDepartment(id)
	if err != nil {
		return err
	}
	if hasUsers {
		return gorm.ErrForeignKeyViolated
	}

	return model.DeleteDepartmentByID(id)
}

// GetDepartment 获取部门详情
func (s *DepartmentService) GetDepartment(id int64) (*model.Department, error) {
	return model.GetDepartmentByID(id)
}

// ListDepartments 分页获取部门列表
func (s *DepartmentService) ListDepartments(page, pageSize int, companyId int64, status int) (*model.DepartmentPage, error) {
	return model.GetDepartmentsByPage(page, pageSize, companyId, status)
}

// GetAllDepartments 获取所有部门（用于下拉选择）
func (s *DepartmentService) GetAllDepartments(companyId int64, status int) ([]*model.Department, error) {
	return model.GetAllDepartments(companyId, status)
}

// GetDepartmentTree 获取公司的部门树
func (s *DepartmentService) GetDepartmentTree(companyId int64) ([]*model.DepartmentTreeNode, error) {
	return model.GetDepartmentTree(companyId)
}

// MoveDepartment 移动部门到新的父部门下
func (s *DepartmentService) MoveDepartment(id int64, newParentId *int64, companyId int64) error {
	return model.MoveDepartment(id, newParentId, companyId)
}

// UpdateDepartmentStatus 更新部门状态
func (s *DepartmentService) UpdateDepartmentStatus(id int64, status int) error {
	return model.UpdateDepartmentStatus(id, status)
}

// GetDescendants 获取部门的所有子孙部门
func (s *DepartmentService) GetDescendants(departmentId int64) ([]*model.Department, error) {
	return model.GetDescendants(departmentId)
}

// GetAncestors 获取部门的所有祖先部门
func (s *DepartmentService) GetAncestors(departmentId int64) ([]*model.Department, error) {
	return model.GetAncestors(departmentId)
}

// GetDepartmentUsers 获取部门下的用户列表
func (s *DepartmentService) GetDepartmentUsers(departmentId int64, page, pageSize int) ([]*model.User, int64, error) {
	var users []*model.User
	var total int64

	query := model.DB.Model(&model.User{}).Where("department_id = ?", departmentId)

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Order("id ASC").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// GetCompanyUsers 获取公司下的用户列表（包括所有部门）
func (s *DepartmentService) GetCompanyUsers(companyId int64, page, pageSize int) ([]*model.User, int64, error) {
	var users []*model.User
	var total int64

	query := model.DB.Model(&model.User{}).Where("company_id = ?", companyId)

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Order("id ASC").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// GetDepartmentStats 获取部门统计信息
func (s *DepartmentService) GetDepartmentStats(id int64) (childCount, userCount int64, err error) {
	// 子部门数量
	var cnt int64
	if err := model.DB.Model(&model.Department{}).Where("parent_id = ?", id).Count(&cnt).Error; err != nil {
		return 0, 0, err
	}
	childCount = cnt

	// 用户数量
	userCount, err = model.GetDepartmentUserCount(id)
	if err != nil {
		return 0, 0, err
	}

	return childCount, userCount, nil
}
