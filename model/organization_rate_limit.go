package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	RateLimitStatusEnabled  = 1
	RateLimitStatusDisabled = 2
)

type OrganizationRateLimit struct {
	Id        int            `json:"id" gorm:"primary_key;autoIncrement"`
	OrgType   string         `json:"org_type" gorm:"type:varchar(32);not null;index:idx_org_rate_limit"`
	OrgId     int            `json:"org_id" gorm:"type:int;not null;index:idx_org_rate_limit"`
	ModelId   int            `json:"model_id" gorm:"type:int;default:0"`
	ModelName string         `json:"model_name" gorm:"type:varchar(128);default:''"`
	TimeSlots string         `json:"time_slots" gorm:"type:text"`
	Rpms      string         `json:"rpms" gorm:"type:text"`
	Priority  int            `json:"priority" gorm:"type:int;default:0"`
	Status    int            `json:"status" gorm:"type:int;default:1"`
	CreatedAt int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (OrganizationRateLimit) TableName() string {
	return "organization_rate_limits"
}

func CreateRateLimit(rule *OrganizationRateLimit) error {
	return DB.Create(rule).Error
}

func GetRateLimitById(id int) (*OrganizationRateLimit, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	var rule OrganizationRateLimit
	err := DB.First(&rule, "id = ?", id).Error
	return &rule, err
}

func UpdateRateLimit(rule *OrganizationRateLimit) error {
	return DB.Save(rule).Error
}

func DeleteRateLimitById(id int) error {
	if id == 0 {
		return errors.New("id is empty")
	}
	return DB.Delete(&OrganizationRateLimit{}, id).Error
}

func GetRateLimitsByOrg(orgType string, orgId int, modelId int, status int, pageInfo *common.PageInfo) ([]*OrganizationRateLimit, int64, error) {
	var rules []*OrganizationRateLimit
	var total int64

	query := DB.Model(&OrganizationRateLimit{}).Where("org_type = ? AND org_id = ?", orgType, orgId)
	if modelId > 0 {
		query = query.Where("model_id = ?", modelId)
	}
	if status > 0 {
		query = query.Where("status = ?", status)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("priority DESC, id ASC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&rules).Error

	return rules, total, err
}

// GetEnabledRateLimitsByOrg returns all enabled rate limit rules for an org (used by cache)
func GetEnabledRateLimitsByOrg(orgType string, orgId int) ([]*OrganizationRateLimit, error) {
	var rules []*OrganizationRateLimit
	err := DB.Where("org_type = ? AND org_id = ? AND status = ?", orgType, orgId, RateLimitStatusEnabled).
		Order("priority DESC, id ASC").
		Find(&rules).Error
	return rules, err
}

// GetDepartmentAncestorIds returns the ancestor department IDs from root to parent, in order
// (root first, then child, ..., then immediate parent)
func GetDepartmentAncestorIds(deptId int) ([]int, error) {
	if deptId == 0 {
		return nil, nil
	}
	dept, err := GetDepartmentById(deptId)
	if err != nil {
		return nil, err
	}
	if dept.ParentId == 0 {
		return nil, nil
	}

	// Parse path to get ancestor IDs
	path := dept.Path
	if path == "" {
		return nil, nil
	}

	var ancestors []int
	// Path format: "1/2/3/" - split and parse
	parts := splitPath(path)
	for _, p := range parts {
		if p > 0 {
			ancestors = append(ancestors, p)
		}
	}
	return ancestors, nil
}

func splitPath(path string) []int {
	if path == "" {
		return nil
	}
	var result []int
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				num := 0
				for _, c := range path[start:i] {
					num = num*10 + int(c-'0')
				}
				if num > 0 {
					result = append(result, num)
				}
			}
			start = i + 1
		}
	}
	return result
}

// GetModelNameById looks up the model name from the model metadata table
func GetModelNameById(modelId int) (string, error) {
	if modelId == 0 {
		return "", nil
	}
	var m Model
	err := DB.Where("id = ?", modelId).Select("model_name").First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return m.ModelName, nil
}

// GetModelIdByName looks up the model ID from the model metadata table
func GetModelIdByName(modelName string) (int, error) {
	if modelName == "" {
		return 0, nil
	}
	var m Model
	err := DB.Where("model_name = ?", modelName).Select("id").First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return m.Id, nil
}

// ValidateModelConsistency checks that modelId and modelName are consistent
func ValidateModelConsistency(modelId int, modelName string) (resolvedId int, resolvedName string, err error) {
	if modelId == 0 && modelName == "" {
		return 0, "", nil
	}

	// If only modelId provided, look up name
	if modelId > 0 && modelName == "" {
		name, err := GetModelNameById(modelId)
		if err != nil {
			return 0, "", err
		}
		return modelId, name, nil
	}

	// If only modelName provided, try to look up id
	if modelId == 0 && modelName != "" {
		id, err := GetModelIdByName(modelName)
		if err != nil {
			return 0, "", err
		}
		return id, modelName, nil
	}

	// Both provided - check consistency
	nameById, err := GetModelNameById(modelId)
	if err != nil {
		return 0, "", err
	}
	if nameById != "" && nameById != modelName {
		return 0, "", errors.New("model_id and model_name do not match")
	}
	return modelId, modelName, nil
}
