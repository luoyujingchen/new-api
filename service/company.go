package service

import (
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// CompanyService 公司服务
type CompanyService struct{}

// NewCompanyService 创建公司服务实例
func NewCompanyService() *CompanyService {
	return &CompanyService{}
}

// CreateCompany 创建公司
func (s *CompanyService) CreateCompany(name, code, description string, status int, sortOrder int, queuePriority int) (*model.Company, error) {
	// 检查代码是否重复
	var cnt int64
	if err := model.DB.Model(&model.Company{}).Where("code = ?", code).Count(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, gorm.ErrDuplicatedKey
	}

	// 检查名称是否重复
	if err := model.DB.Model(&model.Company{}).Where("name = ?", name).Count(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, gorm.ErrDuplicatedKey
	}

	company := &model.Company{
		Name:        name,
		Code:        code,
		Description: description,
		Status:      status,
		SortOrder:   sortOrder,
		QueuePriority: queuePriority,
	}

	if err := company.Insert(); err != nil {
		return nil, err
	}

	return company, nil
}

// UpdateCompany 更新公司
func (s *CompanyService) UpdateCompany(id int64, name, code, description string, status int, sortOrder int, queuePriority int) error {
	company, err := model.GetCompanyByID(id)
	if err != nil {
		return err
	}

	// 检查代码是否重复
	duplicated, err := model.IsCompanyCodeDuplicated(id, code)
	if err != nil {
		return err
	}
	if duplicated {
		return gorm.ErrDuplicatedKey
	}

	// 检查名称是否重复
	duplicated, err = model.IsCompanyNameDuplicated(id, name)
	if err != nil {
		return err
	}
	if duplicated {
		return gorm.ErrDuplicatedKey
	}

	company.Name = name
	company.Code = code
	company.Description = description
	company.Status = status
	company.SortOrder = sortOrder
	company.QueuePriority = queuePriority

	return company.Update()
}

// DeleteCompany 删除公司
func (s *CompanyService) DeleteCompany(id int64) error {
	// 检查是否有关联的部门
	hasDept, err := model.HasDepartments(id)
	if err != nil {
		return err
	}
	if hasDept {
		return gorm.ErrForeignKeyViolated
	}

	// 检查是否有关联的用户
	hasUsers, err := model.HasUsers(id)
	if err != nil {
		return err
	}
	if hasUsers {
		return gorm.ErrForeignKeyViolated
	}

	return model.DeleteCompanyByID(id)
}

// GetCompany 获取公司详情
func (s *CompanyService) GetCompany(id int64) (*model.Company, error) {
	return model.GetCompanyByID(id)
}

// GetCompanyByCode 根据代码获取公司
func (s *CompanyService) GetCompanyByCode(code string) (*model.Company, error) {
	return model.GetCompanyByCode(code)
}

// ListCompanies 分页获取公司列表
func (s *CompanyService) ListCompanies(page, pageSize int, status int) (*model.CompanyPage, error) {
	return model.GetCompaniesByPage(page, pageSize, status)
}

// GetAllCompanies 获取所有公司（用于下拉选择）
func (s *CompanyService) GetAllCompanies(status int) ([]*model.Company, error) {
	return model.GetAllCompanies(status)
}

// UpdateCompanyStatus 更新公司状态
func (s *CompanyService) UpdateCompanyStatus(id int64, status int) error {
	return model.UpdateCompanyStatus(id, status)
}

// GetCompanyStats 获取公司统计信息
func (s *CompanyService) GetCompanyStats(id int64) (deptCount, userCount int64, err error) {
	deptCount, err = model.GetCompanyDepartmentCount(id)
	if err != nil {
		return 0, 0, err
	}
	userCount, err = model.GetCompanyUserCount(id)
	if err != nil {
		return 0, 0, err
	}
	return deptCount, userCount, nil
}
