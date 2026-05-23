package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	DepartmentStatusEnabled  = 1
	DepartmentStatusDisabled = 2
	MaxDepartmentDepth       = 4
)

type Department struct {
	Id          int            `json:"id" gorm:"primary_key;autoIncrement"`
	CompanyId   int            `json:"company_id" gorm:"type:int;not null;index"`
	ParentId    int            `json:"parent_id" gorm:"type:int;default:0;index"`
	Name        string         `json:"name" gorm:"type:varchar(128);not null"`
	Level       int            `json:"level" gorm:"type:int;default:1"`
	Path        string         `json:"path" gorm:"type:varchar(512);default:''"`
	Description string         `json:"description" gorm:"type:varchar(512);default:''"`
	Status      int            `json:"status" gorm:"type:int;default:1"`
	SortOrder   int            `json:"sort_order" gorm:"type:int;default:0"`
	CreatedAt   int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	// Transient fields (not stored in DB)
	CompanyName     string `json:"company_name,omitempty" gorm:"-"`
	ParentName      string `json:"parent_name,omitempty" gorm:"-"`
	ChildCount      int64  `json:"child_count,omitempty" gorm:"-"`
	UserCount       int64  `json:"user_count,omitempty" gorm:"-"`
}

func (Department) TableName() string {
	return "departments"
}

// GetDepartments returns paginated departments with optional filters
func GetDepartments(pageInfo *common.PageInfo, companyId int, status int) ([]*Department, int64, error) {
	var departments []*Department
	var total int64

	query := DB.Model(&Department{})
	if companyId > 0 {
		query = query.Where("company_id = ?", companyId)
	}
	if status > 0 {
		query = query.Where("status = ?", status)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("path asc, sort_order asc, id asc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&departments).Error

	return departments, total, err
}

// GetAllDepartments returns all departments with optional filters
func GetAllDepartments(companyId int, status int) ([]*Department, error) {
	var departments []*Department
	query := DB.Model(&Department{})
	if companyId > 0 {
		query = query.Where("company_id = ?", companyId)
	}
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	err := query.Order("path asc, sort_order asc, id asc").Find(&departments).Error
	return departments, err
}

// GetDepartmentTree returns departments organized as a tree for a specific company
func GetDepartmentTree(companyId int) ([]*Department, error) {
	var departments []*Department
	err := DB.Where("company_id = ?", companyId).
		Order("sort_order asc, id asc").
		Find(&departments).Error
	if err != nil {
		return nil, err
	}
	return departments, nil
}

func GetDepartmentById(id int) (*Department, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	var dept Department
	err := DB.First(&dept, "id = ?", id).Error
	return &dept, err
}

func CreateDepartment(dept *Department) error {
	return DB.Create(dept).Error
}

func UpdateDepartment(dept *Department) error {
	return DB.Save(dept).Error
}

func DeleteDepartmentById(id int) error {
	if id == 0 {
		return errors.New("id is empty")
	}
	return DB.Delete(&Department{}, id).Error
}

// GetDepartmentChildCount returns the number of direct children
func GetDepartmentChildCount(deptId int) (int64, error) {
	var count int64
	err := DB.Model(&Department{}).Where("parent_id = ?", deptId).Count(&count).Error
	return count, err
}

// GetDepartmentUserCount returns the number of users in a department
func GetDepartmentUserCount(deptId int) (int64, error) {
	var count int64
	err := DB.Model(&User{}).Where("department_id = ?", deptId).Count(&count).Error
	return count, err
}

// IsDepartmentNameDuplicate checks if a department name already exists under the same company and parent
func IsDepartmentNameExists(companyId int, parentId int, name string, excludeId int) (bool, error) {
	var count int64
	query := DB.Model(&Department{}).Where("company_id = ? AND parent_id = ? AND name = ?", companyId, parentId, name)
	if excludeId > 0 {
		query = query.Where("id != ?", excludeId)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

// GetDepartmentDepth returns the depth of a department (1-based)
func GetDepartmentDepth(deptId int) (int, error) {
	dept, err := GetDepartmentById(deptId)
	if err != nil {
		return 0, err
	}
	return dept.Level, nil
}

// IsDescendantOf checks if targetId is a descendant of ancestorId
func IsDescendantOf(ancestorId int, targetId int) (bool, error) {
	ancestor, err := GetDepartmentById(ancestorId)
	if err != nil {
		return false, err
	}
	target, err := GetDepartmentById(targetId)
	if err != nil {
		return false, err
	}
	// Check if ancestor's path is a prefix of target's path
	ancestorPath := fmt.Sprintf("%s%d/", ancestor.Path, ancestor.Id)
	return strings.HasPrefix(target.Path, ancestorPath), nil
}

// RecalculatePath recalculates the level and path for a department based on its parent
func RecalculateDepartmentPath(dept *Department) error {
	if dept.ParentId == 0 {
		dept.Level = 1
		dept.Path = ""
		return nil
	}

	parent, err := GetDepartmentById(dept.ParentId)
	if err != nil {
		return fmt.Errorf("parent department not found: %w", err)
	}

	dept.Level = parent.Level + 1
	dept.Path = fmt.Sprintf("%s%d/", parent.Path, parent.Id)
	return nil
}

// GetDepartmentUsers returns users belonging to a department with pagination
func GetDepartmentUsersById(deptId int, pageInfo *common.PageInfo) ([]*User, int64, error) {
	var users []*User
	var total int64

	query := DB.Model(&User{}).Where("department_id = ?", deptId)
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Omit("password").Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&users).Error

	return users, total, err
}

// GetDepartmentsByCompanyId returns all departments for a company (for dropdowns)
func GetDepartmentsByCompanyId(companyId int) ([]*Department, error) {
	var departments []*Department
	err := DB.Where("company_id = ? AND status = ?", companyId, DepartmentStatusEnabled).
		Order("sort_order asc, name asc").
		Find(&departments).Error
	return departments, err
}
