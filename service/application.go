package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"gorm.io/gorm"
)

var (
	ErrApplicationDisabled        = errors.New("application is disabled")
	ErrTokenApplicationNotBound   = errors.New("token is not bound to application")
	ErrTokenApplicationMismatch   = errors.New("token application mismatch")
	ErrApplicationUnrecognized    = errors.New("application is not registered")
	ErrApplicationHeaderAmbiguous = errors.New("application header rules matched multiple applications")
	ErrApplicationHeaderConflict  = errors.New("application header rules conflict with existing application")
	ErrApplicationHeaderInvalid   = errors.New("application header rules are invalid")
)

// ApplicationService 应用服务
type ApplicationService struct{}

// NewApplicationService 创建应用服务实例
func NewApplicationService() *ApplicationService {
	return &ApplicationService{}
}

// CreateApplication 创建应用
func (s *ApplicationService) CreateApplication(name, description string, status int, sortOrder int, headerRules []types.ApplicationHeaderValidationRule) (*model.Application, error) {
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
	if err := application.SetHeaderValidationRules(headerRules); err != nil {
		return nil, err
	}
	if err := s.ValidateApplicationHeaderRuleConflicts(0, application.GetHeaderValidationRules()); err != nil {
		return nil, err
	}

	if err := application.Insert(); err != nil {
		return nil, err
	}

	return application, nil
}

// UpdateApplication 更新应用
func (s *ApplicationService) UpdateApplication(id int64, name, description string, status int, sortOrder int, headerRules *[]types.ApplicationHeaderValidationRule) error {
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
	if headerRules != nil {
		if err := application.SetHeaderValidationRules(*headerRules); err != nil {
			return err
		}
	}
	if err := s.ValidateApplicationHeaderRuleConflicts(id, application.GetHeaderValidationRules()); err != nil {
		return err
	}

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
	if status == 1 {
		application, err := model.GetApplicationByID(id)
		if err != nil {
			return err
		}
		if err := s.ValidateApplicationHeaderRuleConflicts(id, application.GetHeaderValidationRules()); err != nil {
			return err
		}
	}
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

type applicationHeaderValueSet map[string]struct{}

type applicationHeaderRuleConstraints map[string]applicationHeaderValueSet

// ValidateApplicationHeaderRuleConflicts rejects rules that can overlap with
// another application's configured header rules. Only equals/one_of are
// supported, so each constrained header is represented as a finite value set.
// Two rule sets overlap if one request can satisfy both. If they constrain
// different headers, their constraints can coexist on the same request.
func (s *ApplicationService) ValidateApplicationHeaderRuleConflicts(applicationId int64, rules []types.ApplicationHeaderValidationRule) error {
	if len(rules) == 0 {
		return nil
	}
	candidate, err := buildApplicationHeaderRuleConstraints(rules)
	if err != nil {
		return err
	}
	if len(candidate) == 0 {
		return nil
	}

	applications, err := model.GetAllApplications(-1)
	if err != nil {
		return err
	}
	for _, application := range applications {
		if application == nil || application.Id == applicationId {
			continue
		}
		existingRules := application.GetHeaderValidationRules()
		if len(existingRules) == 0 {
			continue
		}
		existing, err := buildApplicationHeaderRuleConstraints(existingRules)
		if err != nil {
			continue
		}
		if applicationHeaderRuleConstraintsOverlap(candidate, existing) {
			return fmt.Errorf("%w: %s", ErrApplicationHeaderConflict, application.Name)
		}
	}
	return nil
}

func buildApplicationHeaderRuleConstraints(rules []types.ApplicationHeaderValidationRule) (applicationHeaderRuleConstraints, error) {
	constraints := make(applicationHeaderRuleConstraints)
	for _, rule := range rules {
		values := applicationHeaderRuleValueSet(rule)
		if len(values) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrApplicationHeaderInvalid, rule.Header)
		}
		existing, ok := constraints[rule.Header]
		if !ok {
			constraints[rule.Header] = values
			continue
		}
		intersection := intersectApplicationHeaderValueSets(existing, values)
		if len(intersection) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrApplicationHeaderInvalid, rule.Header)
		}
		constraints[rule.Header] = intersection
	}
	return constraints, nil
}

func applicationHeaderRuleValueSet(rule types.ApplicationHeaderValidationRule) applicationHeaderValueSet {
	values := make(applicationHeaderValueSet)
	switch rule.Operator {
	case types.ApplicationHeaderOperatorEquals:
		if rule.Value != "" {
			values[rule.Value] = struct{}{}
		}
	case types.ApplicationHeaderOperatorOneOf:
		for _, value := range rule.Values {
			if value != "" {
				values[value] = struct{}{}
			}
		}
	}
	return values
}

func applicationHeaderRuleConstraintsOverlap(a applicationHeaderRuleConstraints, b applicationHeaderRuleConstraints) bool {
	for header, aValues := range a {
		bValues, ok := b[header]
		if !ok {
			continue
		}
		if !applicationHeaderValueSetsOverlap(aValues, bValues) {
			return false
		}
	}
	return true
}

func applicationHeaderValueSetsOverlap(a applicationHeaderValueSet, b applicationHeaderValueSet) bool {
	for value := range a {
		if _, ok := b[value]; ok {
			return true
		}
	}
	return false
}

func intersectApplicationHeaderValueSets(a applicationHeaderValueSet, b applicationHeaderValueSet) applicationHeaderValueSet {
	intersection := make(applicationHeaderValueSet)
	for value := range a {
		if _, ok := b[value]; ok {
			intersection[value] = struct{}{}
		}
	}
	return intersection
}

type RequestApplicationMatch struct {
	Application *model.Application
	Matched     bool
	Checked     bool
	Reason      error
}

// MatchRequestApplicationByHeaders identifies a request application using the
// configured header rules on applications. If no application has header rules,
// Checked is false so callers can keep legacy behavior.
func (s *ApplicationService) MatchRequestApplicationByHeaders(headers http.Header) (*RequestApplicationMatch, error) {
	applications, err := model.GetAllApplications(-1)
	if err != nil {
		return nil, err
	}

	checked := false
	matches := make([]*model.Application, 0, 1)
	for _, application := range applications {
		if application == nil {
			continue
		}
		rules := application.GetHeaderValidationRules()
		if len(rules) == 0 {
			continue
		}
		checked = true
		if types.MatchApplicationHeaderValidationRules(headers, rules) {
			matches = append(matches, application)
		}
	}

	if !checked {
		return &RequestApplicationMatch{Checked: false}, nil
	}
	if len(matches) == 0 {
		return &RequestApplicationMatch{Checked: true, Reason: ErrApplicationUnrecognized}, nil
	}

	enabledMatches := make([]*model.Application, 0, len(matches))
	for _, application := range matches {
		if application.Status != 1 {
			return &RequestApplicationMatch{
				Application: application,
				Matched:     true,
				Checked:     true,
				Reason:      ErrApplicationDisabled,
			}, nil
		}
		enabledMatches = append(enabledMatches, application)
	}
	if len(enabledMatches) > 1 {
		return &RequestApplicationMatch{
			Application: enabledMatches[0],
			Matched:     true,
			Checked:     true,
			Reason:      ErrApplicationHeaderAmbiguous,
		}, nil
	}

	return &RequestApplicationMatch{
		Application: enabledMatches[0],
		Matched:     true,
		Checked:     true,
	}, nil
}
