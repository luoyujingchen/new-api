package model

import (
	"gorm.io/gorm"
)

// Company 子业务公司模型
// 每个公司包含多个部门，用于组织架构管理
type Company struct {
	Id          int64          `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string         `json:"name" gorm:"type:varchar(128);not null;uniqueIndex:uk_company_name,where:deleted_at IS NULL"`
	Code        string         `json:"code" gorm:"type:varchar(32);uniqueIndex:uk_company_code,where:deleted_at IS NULL"` // 公司代码，用于API引用
	Description string         `json:"description,omitempty" gorm:"type:text"`
	Status      int            `json:"status" gorm:"type:int;default:1;index"` // 1=enabled, 0=disabled
	SortOrder   int            `json:"sort_order" gorm:"type:int;default:0"`   // 排序字段
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
	CreatedAt   int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   int64          `json:"updated_at" gorm:"autoUpdateTime"`
	// Relations
	Departments []Department `json:"departments,omitempty" gorm:"foreignKey:CompanyId"`
}

// Insert 新建公司
func (c *Company) Insert() error {
	return DB.Create(c).Error
}

// Update 更新公司
func (c *Company) Update() error {
	return DB.Save(c).Error
}

// DeleteByID 根据ID删除公司
func DeleteCompanyByID(id int64) error {
	return DB.Delete(&Company{}, id).Error
}

// GetCompanyByID 根据ID获取公司
func GetCompanyByID(id int64) (*Company, error) {
	var company Company
	err := DB.First(&company, id).Error
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// GetCompanyByCode 根据代码获取公司
func GetCompanyByCode(code string) (*Company, error) {
	var company Company
	err := DB.Where("code = ?", code).First(&company).Error
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// IsCompanyCodeDuplicated 检查公司代码是否重复（排除自身ID）
func IsCompanyCodeDuplicated(id int64, code string) (bool, error) {
	if code == "" {
		return false, nil
	}
	var cnt int64
	err := DB.Model(&Company{}).Where("code = ? AND id <> ?", code, id).Count(&cnt).Error
	return cnt > 0, err
}

// IsCompanyNameDuplicated 检查公司名称是否重复（排除自身ID）
func IsCompanyNameDuplicated(id int64, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	var cnt int64
	err := DB.Model(&Company{}).Where("name = ? AND id <> ?", name, id).Count(&cnt).Error
	return cnt > 0, err
}

// GetCompanyDepartmentCount 获取公司下的部门数量
func GetCompanyDepartmentCount(companyId int64) (int64, error) {
	var cnt int64
	err := DB.Model(&Department{}).Where("company_id = ?", companyId).Count(&cnt).Error
	return cnt, err
}

// GetCompanyUserCount 获取公司下的用户数量
func GetCompanyUserCount(companyId int64) (int64, error) {
	var cnt int64
	err := DB.Model(&User{}).Where("company_id = ?", companyId).Count(&cnt).Error
	return cnt, err
}

// CompanyPage 分页获取公司列表
type CompanyPage struct {
	Companies []Company `json:"items"`
	Total     int64     `json:"total"`
}

// GetCompaniesByPage 分页获取公司列表
func GetCompaniesByPage(page, pageSize int, status int) (*CompanyPage, error) {
	result := &CompanyPage{}
	query := DB.Model(&Company{})

	if status == 0 || status == 1 {
		query = query.Where("status = ?", status)
	}

	// 获取总数
	if err := query.Count(&result.Total).Error; err != nil {
		return nil, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Order("sort_order ASC, id ASC").Offset(offset).Limit(pageSize).Find(&result.Companies).Error; err != nil {
		return nil, err
	}

	return result, nil
}

// GetAllCompanies 获取所有公司（用于下拉选择等）
func GetAllCompanies(status int) ([]*Company, error) {
	var companies []*Company
	query := DB.Model(&Company{})

	if status == 0 || status == 1 {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("sort_order ASC, id ASC").Find(&companies).Error; err != nil {
		return nil, err
	}
	return companies, nil
}

// UpdateCompanyStatus 更新公司状态
func UpdateCompanyStatus(id int64, status int) error {
	return DB.Model(&Company{}).Where("id = ?", id).Update("status", status).Error
}

// HasDepartments 检查公司是否有关联的部门
func HasDepartments(companyId int64) (bool, error) {
	var cnt int64
	err := DB.Model(&Department{}).Where("company_id = ?", companyId).Count(&cnt).Error
	return cnt > 0, err
}

// HasUsers 检查公司是否有关联的用户
func HasUsers(companyId int64) (bool, error) {
	var cnt int64
	err := DB.Model(&User{}).Where("company_id = ?", companyId).Count(&cnt).Error
	return cnt > 0, err
}
