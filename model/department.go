package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Department 部门模型
// 支持多级部门结构，通过 parent_id 实现层级关系
type Department struct {
	Id          int64          `json:"id" gorm:"primaryKey;autoIncrement"`
	CompanyId   int64          `json:"company_id" gorm:"not null;index:idx_company_parent,priority:1;index:idx_company,priority:1"`
	Name        string         `json:"name" gorm:"type:varchar(128);not null"`
	ParentId    *int64         `json:"parent_id" gorm:"index:idx_company_parent,priority:2"` // 自引用外键，实现层级结构
	Level       int            `json:"level" gorm:"type:int;default:1;index"`                // 部门层级：1-4，根据层级深度自动计算
	Path        string         `json:"path" gorm:"type:varchar(512)"`                         // 层级路径，如 "/1/5/12"，便于查询所有子部门
	Description string         `json:"description,omitempty" gorm:"type:text"`
	Status      int            `json:"status" gorm:"type:int;default:1;index"` // 1=enabled, 0=disabled
	SortOrder   int            `json:"sort_order" gorm:"type:int;default:0"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
	CreatedAt   int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   int64          `json:"updated_at" gorm:"autoUpdateTime"`
	// Relations
	Company  Company      `json:"company,omitempty" gorm:"foreignKey:CompanyId"`
	Parent   *Department `json:"parent,omitempty" gorm:"foreignKey:ParentId"`
	Children []Department `json:"children,omitempty" gorm:"foreignKey:ParentId"`
}

// BeforeCreate GORM hook: 在创建前自动计算 level 和 path
func (d *Department) BeforeCreate(tx *gorm.DB) error {
	if err := d.calculateLevelAndPath(tx); err != nil {
		return err
	}
	return nil
}

// BeforeUpdate GORM hook: 在更新前重新计算 level 和 path（如果 parent_id 变化）
func (d *Department) BeforeUpdate(tx *gorm.DB) error {
	if tx.Statement.Changed("ParentId") || tx.Statement.Changed("CompanyId") {
		if err := d.calculateLevelAndPath(tx); err != nil {
			return err
		}
		// 如果是移动部门，需要更新所有子部门的 path
		if err := d.updateChildrenPaths(tx); err != nil {
			return err
		}
	}
	return nil
}

// calculateLevelAndPath 计算部门的层级和路径
func (d *Department) calculateLevelAndPath(tx *gorm.DB) error {
	if d.ParentId == nil || *d.ParentId == 0 {
		// 根部门
		d.Level = 1
		d.Path = fmt.Sprintf("/%d", d.Id)
		// 如果是新创建，ID可能还是0，需要先获取ID
		if d.Id == 0 {
			// BeforeCreate 时ID还是0，暂不设置path，等插入后再更新
			d.Path = ""
		}
	} else {
		// 子部门，获取父部门信息
		var parent Department
		if err := tx.Where("id = ? AND company_id = ?", *d.ParentId, d.CompanyId).First(&parent).Error; err != nil {
			return fmt.Errorf("parent department not found: %w", err)
		}
		// 限制层级深度为4
		if parent.Level >= 4 {
			return fmt.Errorf("department hierarchy depth cannot exceed 4 levels")
		}
		d.Level = parent.Level + 1
		d.Path = fmt.Sprintf("%s/%d", parent.Path, d.Id)
		// 新创建时ID为0的情况
		if d.Id == 0 {
			d.Path = ""
		}
	}
	return nil
}

// updateChildrenPaths 更新所有子部门的 path
func (d *Department) updateChildrenPaths(tx *gorm.DB) error {
	if d.Id == 0 || d.Path == "" {
		return nil
	}

	// 获取所有直接子部门
	var children []Department
	if err := tx.Where("parent_id = ?", d.Id).Find(&children).Error; err != nil {
		return err
	}

	// 递归更新每个子部门
	for _, child := range children {
		child.Path = fmt.Sprintf("%s/%d", d.Path, child.Id)
		child.Level = d.Level + 1
		if child.Level > 4 {
			return fmt.Errorf("department hierarchy depth cannot exceed 4 levels")
		}
		if err := tx.Save(&child).Error; err != nil {
			return err
		}
		// 递归更新子部门的子部门
		if err := child.updateChildrenPathsRecursively(tx); err != nil {
			return err
		}
	}

	return nil
}

// updateChildrenPathsRecursively 递归更新子部门路径
func (d *Department) updateChildrenPathsRecursively(tx *gorm.DB) error {
	var children []Department
	if err := tx.Where("parent_id = ?", d.Id).Find(&children).Error; err != nil {
		return err
	}

	for _, child := range children {
		child.Path = fmt.Sprintf("%s/%d", d.Path, child.Id)
		child.Level = d.Level + 1
		if child.Level > 4 {
			return fmt.Errorf("department hierarchy depth cannot exceed 4 levels")
		}
		if err := tx.Save(&child).Error; err != nil {
			return err
		}
		if err := child.updateChildrenPathsRecursively(tx); err != nil {
			return err
		}
	}

	return nil
}

// Insert 新建部门
func (d *Department) Insert() error {
	err := DB.Create(d).Error
	if err != nil {
		return err
	}
	// 创建完成后更新 path（因为创建时才有ID）
	if d.Path == "" {
		if d.ParentId == nil || *d.ParentId == 0 {
			d.Path = fmt.Sprintf("/%d", d.Id)
		} else {
			var parent Department
			if err := DB.Where("id = ?", *d.ParentId).First(&parent).Error; err == nil {
				d.Path = fmt.Sprintf("%s/%d", parent.Path, d.Id)
			} else {
				d.Path = fmt.Sprintf("/%d", d.Id)
			}
		}
		DB.Save(d)
	}
	return nil
}

// Update 更新部门
func (d *Department) Update() error {
	return DB.Save(d).Error
}

// DeleteByID 根据ID删除部门
func DeleteDepartmentByID(id int64) error {
	return DB.Delete(&Department{}, id).Error
}

// GetDepartmentByID 根据ID获取部门
func GetDepartmentByID(id int64) (*Department, error) {
	var department Department
	err := DB.Preload("Company").Preload("Parent").First(&department, id).Error
	if err != nil {
		return nil, err
	}
	return &department, nil
}

// IsDepartmentNameDuplicated 检查同一公司下部门名称是否重复（排除自身ID）
func IsDepartmentNameDuplicated(id int64, companyId int64, name string, parentId *int64) (bool, error) {
	if name == "" {
		return false, nil
	}
	var cnt int64
	query := DB.Model(&Department{}).Where("company_id = ? AND name = ? AND id <> ?", companyId, name, id)
	// 如果指定了 parentId，检查同一父部门下的名称重复
	if parentId != nil {
		query = query.Where("parent_id = ?", *parentId)
	} else {
		query = query.Where("parent_id IS NULL")
	}
	err := query.Count(&cnt).Error
	return cnt > 0, err
}

// GetDepartmentUserCount 获取部门下的用户数量
func GetDepartmentUserCount(departmentId int64) (int64, error) {
	var cnt int64
	err := DB.Model(&User{}).Where("department_id = ?", departmentId).Count(&cnt).Error
	return cnt, err
}

// DepartmentPage 分页获取部门列表
type DepartmentPage struct {
	Departments []Department `json:"items"`
	Total       int64        `json:"total"`
}

// GetDepartmentsByPage 分页获取部门列表
func GetDepartmentsByPage(page, pageSize int, companyId int64, status int) (*DepartmentPage, error) {
	result := &DepartmentPage{}
	query := DB.Model(&Department{}).Preload("Company").Preload("Parent")

	if companyId > 0 {
		query = query.Where("company_id = ?", companyId)
	}

	if status == 0 || status == 1 {
		query = query.Where("status = ?", status)
	}

	// 获取总数
	if err := query.Count(&result.Total).Error; err != nil {
		return nil, err
	}

	// 分页查询，按层级和排序字段排序
	offset := (page - 1) * pageSize
	if err := query.Order("company_id ASC, level ASC, sort_order ASC, id ASC").
		Offset(offset).Limit(pageSize).Find(&result.Departments).Error; err != nil {
		return nil, err
	}

	return result, nil
}

// GetAllDepartments 获取所有部门（用于下拉选择等）
func GetAllDepartments(companyId int64, status int) ([]*Department, error) {
	var departments []*Department
	query := DB.Model(&Department{})

	if companyId > 0 {
		query = query.Where("company_id = ?", companyId)
	}

	if status == 0 || status == 1 {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("company_id ASC, level ASC, sort_order ASC, id ASC").Find(&departments).Error; err != nil {
		return nil, err
	}
	return departments, nil
}

// UpdateDepartmentStatus 更新部门状态
func UpdateDepartmentStatus(id int64, status int) error {
	return DB.Model(&Department{}).Where("id = ?", id).Update("status", status).Error
}

// HasChildren 检查部门是否有子部门
func HasChildren(departmentId int64) (bool, error) {
	var cnt int64
	err := DB.Model(&Department{}).Where("parent_id = ?", departmentId).Count(&cnt).Error
	return cnt > 0, err
}

// HasUsersInDepartment 检查部门是否有关联的用户
func HasUsersInDepartment(departmentId int64) (bool, error) {
	var cnt int64
	err := DB.Model(&User{}).Where("department_id = ?", departmentId).Count(&cnt).Error
	return cnt > 0, err
}

// MoveDepartment 移动部门到新的父部门下
func MoveDepartment(id int64, newParentId *int64, companyId int64) error {
	// 获取当前部门
	var dept Department
	if err := DB.First(&dept, id).Error; err != nil {
		return err
	}

	// 不能移动到自己下面
	if newParentId != nil && *newParentId == id {
		return fmt.Errorf("cannot move department to itself")
	}

	maxSubtreeDepth, err := getDepartmentSubtreeDepth(&dept)
	if err != nil {
		return err
	}

	targetLevel := 1

	// 检查新父部门是否在同一公司
	if newParentId != nil {
		var newParent Department
		if err := DB.First(&newParent, *newParentId).Error; err != nil {
			return fmt.Errorf("parent department not found")
		}
		if newParent.CompanyId != companyId {
			return fmt.Errorf("parent department must be in the same company")
		}

		// 检查是否会形成循环（新父部门不能是当前部门的子孙部门）
		if isDescendant(id, *newParentId) {
			return fmt.Errorf("cannot move department to its descendant")
		}

		targetLevel = newParent.Level + 1
	}

	if targetLevel+maxSubtreeDepth > 4 {
		return fmt.Errorf("department hierarchy depth cannot exceed 4 levels")
	}

	// 更新父部门
	dept.ParentId = newParentId
	return DB.Save(&dept).Error
}

// isDescendant 检查 targetId 是否是 sourceId 的子孙部门
func isDescendant(sourceId, targetId int64) bool {
	var target Department
	if err := DB.First(&target, targetId).Error; err != nil {
		return false
	}

	// 检查 sourceId 的 path 是否是 targetId path 的前缀
	var source Department
	if err := DB.First(&source, sourceId).Error; err != nil {
		return false
	}

	// 如果 target 的 path 以 source 的 path 开头，说明 target 是 source 的子孙
	return strings.HasPrefix(target.Path, source.Path+"/")
}

func getDepartmentSubtreeDepth(dept *Department) (int, error) {
	descendants, err := GetDescendants(dept.Id)
	if err != nil {
		return 0, err
	}

	maxDepth := 0
	for _, descendant := range descendants {
		depth := descendant.Level - dept.Level
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth, nil
}

// GetDescendants 获取部门的所有子孙部门
func GetDescendants(departmentId int64) ([]*Department, error) {
	var dept Department
	if err := DB.First(&dept, departmentId).Error; err != nil {
		return nil, err
	}

	var descendants []*Department
	// 使用 path 查询所有子孙部门
	pathPattern := dept.Path + "/%"
	err := DB.Where("path LIKE ?", pathPattern).Order("path ASC").Find(&descendants).Error
	if err != nil {
		return nil, err
	}
	return descendants, nil
}

// GetAncestors 获取部门的所有祖先部门（从根到父）
func GetAncestors(departmentId int64) ([]*Department, error) {
	var dept Department
	if err := DB.First(&dept, departmentId).Error; err != nil {
		return nil, err
	}

	if dept.ParentId == nil {
		return []*Department{}, nil
	}

	// 从 path 中提取所有祖先ID
	pathParts := strings.Split(strings.Trim(dept.Path, "/"), "/")
	if len(pathParts) <= 1 {
		return []*Department{}, nil
	}

	// 排除当前部门ID，只取祖先ID
	ancestorIds := make([]int64, 0, len(pathParts)-1)
	for _, part := range pathParts[:len(pathParts)-1] {
		if id := parseInt64(part); id > 0 {
			ancestorIds = append(ancestorIds, id)
		}
	}

	if len(ancestorIds) == 0 {
		return []*Department{}, nil
	}

	var ancestors []*Department
	err := DB.Where("id IN ?", ancestorIds).Order("path ASC").Find(&ancestors).Error
	if err != nil {
		return nil, err
	}
	return ancestors, nil
}

// GetDepartmentTree 获取公司的部门树
func GetDepartmentTree(companyId int64) ([]*DepartmentTreeNode, error) {
	// 获取公司所有部门
	var departments []*Department
	err := DB.Where("company_id = ?", companyId).Order("level ASC, sort_order ASC, id ASC").Find(&departments).Error
	if err != nil {
		return nil, err
	}

	// 构建树形结构
	return buildDepartmentTree(departments, 0), nil
}

// DepartmentTreeNode 部门树节点
type DepartmentTreeNode struct {
	Department
	Children []*DepartmentTreeNode `json:"children,omitempty"`
}

// buildDepartmentTree 构建部门树
func buildDepartmentTree(departments []*Department, parentId int64) []*DepartmentTreeNode {
	var nodes []*DepartmentTreeNode

	for _, dept := range departments {
		var deptParentId int64 = 0
		if dept.ParentId != nil {
			deptParentId = *dept.ParentId
		}

		if deptParentId == parentId {
			node := &DepartmentTreeNode{
				Department: *dept,
				Children:   buildDepartmentTree(departments, dept.Id),
			}
			nodes = append(nodes, node)
		}
	}

	return nodes
}

// parseInt64 字符串转int64辅助函数
func parseInt64(s string) int64 {
	var id int64
	if _, err := fmt.Sscanf(s, "%d", &id); err != nil {
		return 0
	}
	return id
}
