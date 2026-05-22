package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var (
	ErrApplicationDisabled       = errors.New("application is disabled")
	ErrTokenApplicationNotBound  = errors.New("token is not bound to application")
	ErrTokenApplicationMismatch  = errors.New("token application mismatch")
)

// ApplicationService 应用服务
type ApplicationService struct{}

// NewApplicationService 创建应用服务实例
func NewApplicationService() *ApplicationService {
	return &ApplicationService{}
}

// CreateApplication 创建应用
func (s *ApplicationService) CreateApplication(name, description string, status int, sortOrder int) (*model.Application, error) {
	// 检查名称是否重复
	var cnt int64
	if err := model.DB.Model(&model.Application{}).Where("name = ?", name).Count(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, gorm.ErrDuplicatedKey
	}

	// 生成应用唯一标识
	appKey, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}

	application := &model.Application{
		AppKey:      appKey,
		Name:        name,
		Description: description,
		Status:      status,
		SortOrder:   sortOrder,
	}

	if err := application.Insert(); err != nil {
		return nil, err
	}

	return application, nil
}

// UpdateApplication 更新应用
func (s *ApplicationService) UpdateApplication(id int64, name, description string, status int, sortOrder int) error {
	application, err := model.GetApplicationByID(id)
	if err != nil {
		return err
	}

	// 检查名称是否重复
	duplicated, err := model.IsApplicationNameDuplicated(id, name)
	if err != nil {
		return err
	}
	if duplicated {
		return gorm.ErrDuplicatedKey
	}

	application.Name = name
	application.Description = description
	application.Status = status
	application.SortOrder = sortOrder

	return application.Update()
}

// DeleteApplication 删除应用
func (s *ApplicationService) DeleteApplication(id int64) error {
	// 检查是否有关联的令牌
	hasTokens, err := model.HasTokens(id)
	if err != nil {
		return err
	}
	if hasTokens {
		return gorm.ErrForeignKeyViolated
	}

	return model.DeleteApplicationByID(id)
}

// GetApplication 获取应用详情
func (s *ApplicationService) GetApplication(id int64) (*model.Application, error) {
	return model.GetApplicationByID(id)
}

// ListApplications 分页获取应用列表
func (s *ApplicationService) ListApplications(page, pageSize int, status int) (*model.ApplicationPage, error) {
	return model.GetApplicationsByPage(page, pageSize, status)
}

// GetAllApplications 获取所有应用（用于下拉选择）
func (s *ApplicationService) GetAllApplications(status int) ([]*model.Application, error) {
	return model.GetAllApplications(status)
}

// UpdateApplicationStatus 更新应用状态
func (s *ApplicationService) UpdateApplicationStatus(id int64, status int) error {
	return model.UpdateApplicationStatus(id, status)
}

// GetApplicationStats 获取应用统计信息
func (s *ApplicationService) GetApplicationStats(id int64) (tokenCount int64, err error) {
	tokenCount, err = model.GetApplicationTokenCount(id)
	if err != nil {
		return 0, err
	}
	return tokenCount, nil
}

// GetApplicationStatsBatch 批量获取应用统计信息。
func (s *ApplicationService) GetApplicationStatsBatch(ids []int64) (map[int64]int64, error) {
	return model.GetApplicationTokenCounts(ids)
}

// ValidateApplicationSelection 校验令牌绑定的应用是否合法且可用。
func (s *ApplicationService) ValidateApplicationSelection(applicationId *int64) (*model.Application, error) {
	if applicationId == nil {
		return nil, nil
	}

	application, err := model.GetApplicationByID(*applicationId)
	if err != nil {
		return nil, err
	}
	if application.Status != 1 {
		return nil, ErrApplicationDisabled
	}
	return application, nil
}

// ValidateRequestApplication 校验请求头中的 x-app-id 是否与令牌绑定应用匹配。
func (s *ApplicationService) ValidateRequestApplication(token *model.Token, appKey string) (*model.Application, error) {
	if token == nil {
		return nil, errors.New("token is nil")
	}
	appKey = strings.TrimSpace(appKey)
	if appKey == "" {
		return nil, nil
	}
	if token.ApplicationId == nil {
		return nil, ErrTokenApplicationNotBound
	}

	application, err := s.ValidateApplicationSelection(token.ApplicationId)
	if err != nil {
		return nil, err
	}
	if application == nil || application.AppKey != appKey {
		return nil, ErrTokenApplicationMismatch
	}

	return application, nil
}
