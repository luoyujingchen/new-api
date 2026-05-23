package model

import (
	"errors"

	"gorm.io/gorm"
)

// SetUserDepartment sets the company and department for a user
func SetUserDepartment(userId int, companyId int, departmentId int) error {
	if userId == 0 {
		return errors.New("user id is empty")
	}
	return DB.Model(&User{}).Where("id = ?", userId).Updates(map[string]interface{}{
		"company_id":    companyId,
		"department_id": departmentId,
	}).Error
}

// ClearUserDepartment clears both company and department for a user
func ClearUserDepartment(userId int) error {
	if userId == 0 {
		return errors.New("user id is empty")
	}
	return DB.Model(&User{}).Where("id = ?", userId).Updates(map[string]interface{}{
		"company_id":    0,
		"department_id": 0,
	}).Error
}

// GetUserCompanyAndDepartment gets the company and department info for a user
func GetUserCompanyAndDepartment(userId int) (companyId int, departmentId int, err error) {
	var user User
	err = DB.Where("id = ?", userId).Select("company_id, department_id").First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	return user.CompanyId, user.DepartmentId, nil
}
