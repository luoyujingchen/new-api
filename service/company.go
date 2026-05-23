package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

// Company service errors
var (
	ErrCompanyNameExists = errors.New("company name already exists")
	ErrCompanyCodeExists = errors.New("company code already exists")
	ErrCompanyHasDepts   = errors.New("company still has departments, cannot delete")
	ErrCompanyHasUsers   = errors.New("company still has users, cannot delete")
	ErrCompanyNotFound   = errors.New("company not found")
)

func GetCompanyResponse(company *model.Company) (*dto.CompanyResponse, error) {
	deptCount, err := model.GetCompanyDepartmentCount(company.Id)
	if err != nil {
		deptCount = 0
	}
	userCount, err := model.GetCompanyUserCount(company.Id)
	if err != nil {
		userCount = 0
	}

	return &dto.CompanyResponse{
		Id:              company.Id,
		Name:            company.Name,
		Code:            company.Code,
		Description:     company.Description,
		Status:          company.Status,
		SortOrder:       company.SortOrder,
		DepartmentCount: deptCount,
		UserCount:       userCount,
		CreatedAt:       company.CreatedAt,
		UpdatedAt:       company.UpdatedAt,
	}, nil
}

func CreateCompany(req *dto.CompanyRequest) (*model.Company, error) {
	// Check name uniqueness
	exists, err := model.IsCompanyNameExists(req.Name, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrCompanyNameExists
	}

	// Check code uniqueness
	exists, err = model.IsCompanyCodeExists(req.Code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrCompanyCodeExists
	}

	company := &model.Company{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Status:      model.CompanyStatusEnabled,
		SortOrder:   0,
	}
	if req.Status != nil {
		company.Status = *req.Status
	}
	if req.SortOrder != nil {
		company.SortOrder = *req.SortOrder
	}

	err = model.CreateCompany(company)
	return company, err
}

func UpdateCompanyService(id int, req *dto.CompanyRequest) (*model.Company, error) {
	company, err := model.GetCompanyById(id)
	if err != nil {
		return nil, ErrCompanyNotFound
	}

	// Check name uniqueness
	exists, err := model.IsCompanyNameExists(req.Name, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrCompanyNameExists
	}

	// Check code uniqueness
	exists, err = model.IsCompanyCodeExists(req.Code, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrCompanyCodeExists
	}

	company.Name = req.Name
	company.Code = req.Code
	company.Description = req.Description
	if req.Status != nil {
		company.Status = *req.Status
	}
	if req.SortOrder != nil {
		company.SortOrder = *req.SortOrder
	}

	err = model.UpdateCompany(company)
	return company, err
}

func DeleteCompanyService(id int) error {
	// Check if company has departments
	deptCount, err := model.GetCompanyDepartmentCount(id)
	if err != nil {
		return err
	}
	if deptCount > 0 {
		return ErrCompanyHasDepts
	}

	// Check if company has users
	userCount, err := model.GetCompanyUserCount(id)
	if err != nil {
		return err
	}
	if userCount > 0 {
		return ErrCompanyHasUsers
	}

	return model.DeleteCompanyById(id)
}

func UpdateCompanyStatusService(id int, status int) error {
	company, err := model.GetCompanyById(id)
	if err != nil {
		return ErrCompanyNotFound
	}
	company.Status = status
	return model.UpdateCompany(company)
}

func GetCompanyUsersService(id int, pageInfo *common.PageInfo) ([]*model.User, int64, error) {
	_, err := model.GetCompanyById(id)
	if err != nil {
		return nil, 0, ErrCompanyNotFound
	}
	return model.GetCompanyUsersById(id, pageInfo)
}
