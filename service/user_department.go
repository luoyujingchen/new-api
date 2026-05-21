package service

import (
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// UserDepartmentService 用户部门服务
type UserDepartmentService struct {
}

// NewUserDepartmentService 创建用户部门服务实例
func NewUserDepartmentService() *UserDepartmentService {
	return &UserDepartmentService{}
}

// SetUserDepartment 设置用户的公司和部门
func (s *UserDepartmentService) SetUserDepartment(userId int, companyId *int64, departmentId *int64) error {
	// 直接使用 model.DB 更新
	updateData := make(map[string]interface{})

	if departmentId != nil && *departmentId > 0 {
		// 验证部门存在
		dept, err := model.GetDepartmentByID(*departmentId)
		if err != nil {
			return gorm.ErrRecordNotFound
		}

		// 如果同时设置了公司，验证部门属于该公司
		if companyId != nil && *companyId > 0 && dept.CompanyId != *companyId {
			return gorm.ErrInvalidData
		}

		// 如果只设置了部门没设置公司，自动设置公司
		if companyId == nil || *companyId == 0 {
			companyId = &dept.CompanyId
		}

		updateData["company_id"] = companyId
		updateData["department_id"] = departmentId
	} else {
		// 没有设置部门，清空部门
		updateData["department_id"] = nil
		// 如果设置了公司，只设置公司
		if companyId != nil && *companyId > 0 {
			// 验证公司存在
			_, err := model.GetCompanyByID(*companyId)
			if err != nil {
				return gorm.ErrRecordNotFound
			}
			updateData["company_id"] = companyId
		} else {
			updateData["company_id"] = nil
		}
	}

	result := model.DB.Model(&model.User{}).Where("id = ?", userId).Updates(updateData)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ClearUserDepartment 清空用户的公司和部门
func (s *UserDepartmentService) ClearUserDepartment(userId int) error {
	result := model.DB.Model(&model.User{}).Where("id = ?", userId).
		Updates(map[string]interface{}{
			"company_id":    nil,
			"department_id": nil,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetUserDepartment 获取用户的部门和公司信息
func (s *UserDepartmentService) GetUserDepartment(userId int) (companyId *int64, departmentId *int64, err error) {
	var user model.User
	if err := model.DB.Select("company_id, department_id").First(&user, userId).Error; err != nil {
		return nil, nil, err
	}
	return user.CompanyId, user.DepartmentId, nil
}

// GetDepartmentUsers 获取部门下的用户列表
func (s *UserDepartmentService) GetDepartmentUsers(departmentId int64, page, pageSize int) ([]*model.User, int64, error) {
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

// GetCompanyUsers 获取公司下的用户列表（包括所有部门和无部门用户）
func (s *UserDepartmentService) GetCompanyUsers(companyId int64, page, pageSize int) ([]*model.User, int64, error) {
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

// BatchSetUserDepartment 批量设置用户的部门和公司
func (s *UserDepartmentService) BatchSetUserDepartment(userIds []int, companyId *int64, departmentId *int64) error {
	if len(userIds) == 0 {
		return nil
	}

	// 如果设置了部门，验证部门存在
	if departmentId != nil && *departmentId > 0 {
		dept, err := model.GetDepartmentByID(*departmentId)
		if err != nil {
			return gorm.ErrRecordNotFound
		}

		// 验证公司关联
		if companyId != nil && *companyId > 0 && dept.CompanyId != *companyId {
			return gorm.ErrInvalidData
		}

		// 更新用户的公司和部门
		updateData := map[string]interface{}{
			"department_id": departmentId,
			"company_id":    dept.CompanyId,
		}
		return model.DB.Model(&model.User{}).Where("id IN ?", userIds).Updates(updateData).Error
	}

	// 只设置公司
	if companyId != nil && *companyId > 0 {
		_, err := model.GetCompanyByID(*companyId)
		if err != nil {
			return gorm.ErrRecordNotFound
		}
		updateData := map[string]interface{}{
			"company_id":    companyId,
			"department_id": nil,
		}
		return model.DB.Model(&model.User{}).Where("id IN ?", userIds).Updates(updateData).Error
	}

	// 清空公司和部门
	updateData := map[string]interface{}{
		"company_id":    nil,
		"department_id": nil,
	}
	return model.DB.Model(&model.User{}).Where("id IN ?", userIds).Updates(updateData).Error
}

// GetUserDepartmentInfo 获取用户的完整部门和公司信息（带关联数据）
func (s *UserDepartmentService) GetUserDepartmentInfo(userId int) (*model.User, error) {
	var user model.User
	if err := model.DB.Preload("Company").Preload("Department").First(&user, userId).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
