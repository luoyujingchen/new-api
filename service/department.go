package service

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

// Department service errors
var (
	ErrDepartmentNotFound       = errors.New("department not found")
	ErrDepartmentNameExists     = errors.New("department name already exists under the same parent")
	ErrDepartmentParentNotFound = errors.New("parent department not found")
	ErrDepartmentParentMismatch = errors.New("parent department does not belong to the same company")
	ErrDepartmentMaxDepth       = errors.New("maximum department depth exceeded (max 4 levels)")
	ErrDepartmentHasChildren    = errors.New("department still has sub-departments, cannot delete")
	ErrDepartmentHasUsers       = errors.New("department still has users, cannot delete")
	ErrDepartmentSelfMove       = errors.New("cannot move department under itself")
	ErrDepartmentDescendantMove = errors.New("cannot move department under its own descendant")
	ErrDepartmentCompanyChange  = errors.New("cannot change the company of a department")
)

func GetDepartmentResponse(dept *model.Department) (*dto.DepartmentResponse, error) {
	resp := &dto.DepartmentResponse{
		Id:          dept.Id,
		CompanyId:   dept.CompanyId,
		ParentId:    dept.ParentId,
		Name:        dept.Name,
		Level:       dept.Level,
		Path:        dept.Path,
		Description: dept.Description,
		Status:      dept.Status,
		SortOrder:   dept.SortOrder,
		CreatedAt:   dept.CreatedAt,
		UpdatedAt:   dept.UpdatedAt,
	}

	// Get child count
	childCount, err := model.GetDepartmentChildCount(dept.Id)
	if err == nil {
		resp.ChildCount = childCount
	}

	// Get user count
	userCount, err := model.GetDepartmentUserCount(dept.Id)
	if err == nil {
		resp.UserCount = userCount
	}

	// Get company name
	company, err := model.GetCompanyById(dept.CompanyId)
	if err == nil {
		resp.CompanyName = company.Name
	}

	// Get parent name
	if dept.ParentId > 0 {
		parent, err := model.GetDepartmentById(dept.ParentId)
		if err == nil {
			resp.ParentName = parent.Name
		}
	}

	return resp, nil
}

func CreateDepartmentService(req *dto.DepartmentRequest) (*model.Department, error) {
	// Verify company exists
	_, err := model.GetCompanyById(req.CompanyId)
	if err != nil {
		return nil, ErrCompanyNotFound
	}

	// Determine parent and validate
	parentId := 0
	if req.ParentId != nil && *req.ParentId > 0 {
		parentId = *req.ParentId

		// Verify parent exists and belongs to same company
		parent, err := model.GetDepartmentById(parentId)
		if err != nil {
			return nil, ErrDepartmentParentNotFound
		}
		if parent.CompanyId != req.CompanyId {
			return nil, ErrDepartmentParentMismatch
		}

		// Check max depth
		if parent.Level >= model.MaxDepartmentDepth {
			return nil, ErrDepartmentMaxDepth
		}
	}

	// Check name uniqueness under same company+parent
	exists, err := model.IsDepartmentNameExists(req.CompanyId, parentId, req.Name, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDepartmentNameExists
	}

	dept := &model.Department{
		CompanyId:   req.CompanyId,
		ParentId:    parentId,
		Name:        req.Name,
		Description: req.Description,
		Status:      model.DepartmentStatusEnabled,
		SortOrder:   0,
	}
	if req.Status != nil {
		dept.Status = *req.Status
	}
	if req.SortOrder != nil {
		dept.SortOrder = *req.SortOrder
	}

	// Calculate level and path
	if err := model.RecalculateDepartmentPath(dept); err != nil {
		return nil, err
	}

	err = model.CreateDepartment(dept)
	return dept, err
}

func UpdateDepartmentService(id int, req *dto.DepartmentRequest) (*model.Department, error) {
	dept, err := model.GetDepartmentById(id)
	if err != nil {
		return nil, ErrDepartmentNotFound
	}

	// Do not allow changing company
	if req.CompanyId != dept.CompanyId {
		return nil, ErrDepartmentCompanyChange
	}

	// Determine effective parent
	parentId := dept.ParentId
	if req.ParentId != nil {
		parentId = *req.ParentId
	}

	// If changing parent, validate
	if req.ParentId != nil && *req.ParentId != dept.ParentId {
		newParentId := *req.ParentId
		if newParentId > 0 {
			parent, err := model.GetDepartmentById(newParentId)
			if err != nil {
				return nil, ErrDepartmentParentNotFound
			}
			if parent.CompanyId != dept.CompanyId {
				return nil, ErrDepartmentParentMismatch
			}
		}

		// Use move logic for parent changes
		return MoveDepartmentService(id, &dto.DepartmentMoveRequest{TargetParentId: newParentId})
	}

	// Check name uniqueness
	exists, err := model.IsDepartmentNameExists(dept.CompanyId, parentId, req.Name, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDepartmentNameExists
	}

	dept.Name = req.Name
	dept.Description = req.Description
	if req.Status != nil {
		dept.Status = *req.Status
	}
	if req.SortOrder != nil {
		dept.SortOrder = *req.SortOrder
	}

	err = model.UpdateDepartment(dept)
	return dept, err
}

func MoveDepartmentService(id int, req *dto.DepartmentMoveRequest) (*model.Department, error) {
	dept, err := model.GetDepartmentById(id)
	if err != nil {
		return nil, ErrDepartmentNotFound
	}

	targetParentId := req.TargetParentId

	// Cannot move to self
	if targetParentId == id {
		return nil, ErrDepartmentSelfMove
	}

	// Cannot move to descendant
	if targetParentId > 0 {
		isDesc, err := model.IsDescendantOf(id, targetParentId)
		if err != nil {
			return nil, err
		}
		if isDesc {
			return nil, ErrDepartmentDescendantMove
		}

		// Verify parent exists and belongs to same company
		parent, err := model.GetDepartmentById(targetParentId)
		if err != nil {
			return nil, ErrDepartmentParentNotFound
		}
		if parent.CompanyId != dept.CompanyId {
			return nil, ErrDepartmentParentMismatch
		}

		// Check max depth: new parent level + 1 must be <= max
		if parent.Level+1 > model.MaxDepartmentDepth {
			return nil, ErrDepartmentMaxDepth
		}
	}

	dept.ParentId = targetParentId
	if err := model.RecalculateDepartmentPath(dept); err != nil {
		return nil, err
	}

	err = model.UpdateDepartment(dept)
	return dept, err
}

func DeleteDepartmentService(id int) error {
	// Check for sub-departments
	childCount, err := model.GetDepartmentChildCount(id)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return ErrDepartmentHasChildren
	}

	// Check for users
	userCount, err := model.GetDepartmentUserCount(id)
	if err != nil {
		return err
	}
	if userCount > 0 {
		return ErrDepartmentHasUsers
	}

	return model.DeleteDepartmentById(id)
}

func UpdateDepartmentStatusService(id int, status int) error {
	dept, err := model.GetDepartmentById(id)
	if err != nil {
		return ErrDepartmentNotFound
	}
	dept.Status = status
	return model.UpdateDepartment(dept)
}

func GetDepartmentUsersService(id int, pageInfo *common.PageInfo) ([]*model.User, int64, error) {
	_, err := model.GetDepartmentById(id)
	if err != nil {
		return nil, 0, ErrDepartmentNotFound
	}
	return model.GetDepartmentUsersById(id, pageInfo)
}

// SetUserDepartmentService handles setting or clearing a user's department affiliation
func SetUserDepartmentService(userId int, req *dto.UserDepartmentRequest) error {
	companyId := 0
	departmentId := 0

	if req.CompanyId != nil {
		companyId = *req.CompanyId
	}
	if req.DepartmentId != nil {
		departmentId = *req.DepartmentId
	}

	// If both are zero, clear the affiliation
	if companyId == 0 && departmentId == 0 {
		return model.ClearUserDepartment(userId)
	}

	// If department is provided without company, auto-fill from department
	if departmentId > 0 && companyId == 0 {
		dept, err := model.GetDepartmentById(departmentId)
		if err != nil {
			return ErrDepartmentNotFound
		}
		companyId = dept.CompanyId
	}

	// If company is provided, verify it exists
	if companyId > 0 {
		_, err := model.GetCompanyById(companyId)
		if err != nil {
			return ErrCompanyNotFound
		}
	}

	// If department is provided, verify it belongs to the company
	if departmentId > 0 {
		dept, err := model.GetDepartmentById(departmentId)
		if err != nil {
			return ErrDepartmentNotFound
		}
		if dept.CompanyId != companyId {
			return ErrDepartmentParentMismatch
		}
	}

	return model.SetUserDepartment(userId, companyId, departmentId)
}

// fmt import guard - ensure it's available for error formatting
var _ = fmt.Sprintf
