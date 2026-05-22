package model

import (
	"gorm.io/gorm"
)

// Application 应用模型
// 用于管理员创建应用，用户创建 API Key 时可以选择关联应用
type Application struct {
	Id          int64          `json:"id" gorm:"primaryKey;autoIncrement"`
	AppKey      string         `json:"app_key" gorm:"type:varchar(64);uniqueIndex;not null"` // 应用唯一标识，用于API调用
	Name        string         `json:"name" gorm:"type:varchar(128);not null;uniqueIndex:uk_application_name,where:deleted_at IS NULL"`
	Description string         `json:"description,omitempty" gorm:"type:text"`
	Status      int            `json:"status" gorm:"type:int;default:1;index"` // 1=enabled, 0=disabled
	SortOrder   int            `json:"sort_order" gorm:"type:int;default:0"`   // 排序字段
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
	CreatedAt   int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   int64          `json:"updated_at" gorm:"autoUpdateTime"`
	// Relations
	Tokens []Token `json:"tokens,omitempty" gorm:"foreignKey:ApplicationId"`
}

// Insert 新建应用
func (a *Application) Insert() error {
	return DB.Create(a).Error
}

// Update 更新应用
func (a *Application) Update() error {
	return DB.Save(a).Error
}

// DeleteByID 根据ID删除应用
func DeleteApplicationByID(id int64) error {
	return DB.Delete(&Application{}, id).Error
}

// GetApplicationByID 根据ID获取应用
func GetApplicationByID(id int64) (*Application, error) {
	var application Application
	err := DB.First(&application, id).Error
	if err != nil {
		return nil, err
	}
	return &application, nil
}

// IsApplicationNameDuplicated 检查应用名称是否重复（排除自身ID）
func IsApplicationNameDuplicated(id int64, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	var cnt int64
	err := DB.Model(&Application{}).Where("name = ? AND id <> ?", name, id).Count(&cnt).Error
	return cnt > 0, err
}

// GetApplicationTokenCount 获取应用关联的令牌数量
func GetApplicationTokenCount(applicationId int64) (int64, error) {
	var cnt int64
	err := DB.Model(&Token{}).Where("application_id = ?", applicationId).Count(&cnt).Error
	return cnt, err
}

type applicationTokenCountRow struct {
	ApplicationId int64 `gorm:"column:application_id"`
	TokenCount    int64 `gorm:"column:token_count"`
}

// GetApplicationTokenCounts 获取多个应用关联的令牌数量。
func GetApplicationTokenCounts(applicationIds []int64) (map[int64]int64, error) {
	counts := make(map[int64]int64, len(applicationIds))
	if len(applicationIds) == 0 {
		return counts, nil
	}

	var rows []applicationTokenCountRow
	err := DB.Model(&Token{}).
		Select("application_id, COUNT(*) as token_count").
		Where("application_id IN ?", applicationIds).
		Group("application_id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		counts[row.ApplicationId] = row.TokenCount
	}
	return counts, nil
}

// ApplicationPage 分页获取应用列表
type ApplicationPage struct {
	Applications []Application `json:"items"`
	Total        int64         `json:"total"`
}

// GetApplicationsByPage 分页获取应用列表
func GetApplicationsByPage(page, pageSize int, status int) (*ApplicationPage, error) {
	result := &ApplicationPage{}
	query := DB.Model(&Application{})

	if status == 0 || status == 1 {
		query = query.Where("status = ?", status)
	}

	// 获取总数
	if err := query.Count(&result.Total).Error; err != nil {
		return nil, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Order("sort_order ASC, id ASC").Offset(offset).Limit(pageSize).Find(&result.Applications).Error; err != nil {
		return nil, err
	}

	return result, nil
}

// GetAllApplications 获取所有应用（用于下拉选择等）
func GetAllApplications(status int) ([]*Application, error) {
	var applications []*Application
	query := DB.Model(&Application{})

	if status == 0 || status == 1 {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("sort_order ASC, id ASC").Find(&applications).Error; err != nil {
		return nil, err
	}
	return applications, nil
}

// UpdateApplicationStatus 更新应用状态
func UpdateApplicationStatus(id int64, status int) error {
	return DB.Model(&Application{}).Where("id = ?", id).Update("status", status).Error
}

// HasTokens 检查应用是否有关联的令牌
func HasTokens(applicationId int64) (bool, error) {
	var cnt int64
	err := DB.Model(&Token{}).Where("application_id = ?", applicationId).Count(&cnt).Error
	return cnt > 0, err
}
