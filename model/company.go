package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	CompanyStatusEnabled  = 1
	CompanyStatusDisabled = 2
)

type Company struct {
	Id          int            `json:"id" gorm:"primary_key;autoIncrement"`
	Name        string         `json:"name" gorm:"type:varchar(128);uniqueIndex;not null"`
	Code        string         `json:"code" gorm:"type:varchar(64);uniqueIndex;not null"`
	Description string         `json:"description" gorm:"type:varchar(512);default:''"`
	Status      int            `json:"status" gorm:"type:int;default:1"`
	SortOrder   int            `json:"sort_order" gorm:"type:int;default:0"`
	CreatedAt   int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Company) TableName() string {
	return "companies"
}

func GetCompanies(pageInfo *common.PageInfo, status int) ([]*Company, int64, error) {
	var companies []*Company
	var total int64

	query := DB.Model(&Company{})
	if status > 0 {
		query = query.Where("status = ?", status)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("sort_order asc, id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&companies).Error

	return companies, total, err
}

func GetAllCompanies(status int) ([]*Company, error) {
	var companies []*Company
	query := DB.Model(&Company{})
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	err := query.Order("sort_order asc, id desc").Find(&companies).Error
	return companies, err
}

func GetCompanyById(id int) (*Company, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	var company Company
	err := DB.First(&company, "id = ?", id).Error
	return &company, err
}

func CreateCompany(company *Company) error {
	return DB.Create(company).Error
}

func UpdateCompany(company *Company) error {
	return DB.Save(company).Error
}

func DeleteCompanyById(id int) error {
	if id == 0 {
		return errors.New("id is empty")
	}
	return DB.Delete(&Company{}, id).Error
}

func GetCompanyDepartmentCount(companyId int) (int64, error) {
	var count int64
	err := DB.Model(&Department{}).Where("company_id = ?", companyId).Count(&count).Error
	return count, err
}

func GetCompanyUserCount(companyId int) (int64, error) {
	var count int64
	err := DB.Model(&User{}).Where("company_id = ?", companyId).Count(&count).Error
	return count, err
}

func IsCompanyNameExists(name string, excludeId int) (bool, error) {
	var count int64
	query := DB.Model(&Company{}).Where("name = ?", name)
	if excludeId > 0 {
		query = query.Where("id != ?", excludeId)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

func IsCompanyCodeExists(code string, excludeId int) (bool, error) {
	var count int64
	query := DB.Model(&Company{}).Where("code = ?", code)
	if excludeId > 0 {
		query = query.Where("id != ?", excludeId)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

func GetCompanyUsersById(companyId int, pageInfo *common.PageInfo) ([]*User, int64, error) {
	var users []*User
	var total int64

	query := DB.Model(&User{}).Where("company_id = ?", companyId)
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
