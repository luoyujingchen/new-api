package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

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

const applicationHeaderRulesCacheTTL = 30 * time.Second

type cachedApplicationHeaderRule struct {
	Application model.Application
	Rules       []types.ApplicationHeaderValidationRule
}

var applicationHeaderRulesCache = struct {
	sync.RWMutex
	expiresAt time.Time
	rules     []cachedApplicationHeaderRule
}{}

func GetApplicationHeaderDetectionMode() string {
	common.OptionMapRWMutex.RLock()
	mode := common.OptionMap["ApplicationHeaderDetectionMode"]
	common.OptionMapRWMutex.RUnlock()
	return types.NormalizeApplicationHeaderDetectionMode(mode)
}

func InvalidateApplicationHeaderRulesCache() {
	applicationHeaderRulesCache.Lock()
	applicationHeaderRulesCache.expiresAt = time.Time{}
	applicationHeaderRulesCache.rules = nil
	applicationHeaderRulesCache.Unlock()
}

func (s *ApplicationService) getCachedApplicationHeaderRules() ([]cachedApplicationHeaderRule, error) {
	now := time.Now()
	applicationHeaderRulesCache.RLock()
	if now.Before(applicationHeaderRulesCache.expiresAt) {
		rules := cloneCachedApplicationHeaderRules(applicationHeaderRulesCache.rules)
		applicationHeaderRulesCache.RUnlock()
		return rules, nil
	}
	applicationHeaderRulesCache.RUnlock()

	applicationHeaderRulesCache.Lock()
	defer applicationHeaderRulesCache.Unlock()
	if now.Before(applicationHeaderRulesCache.expiresAt) {
		return cloneCachedApplicationHeaderRules(applicationHeaderRulesCache.rules), nil
	}

	applications, err := model.GetAllApplications(1)
	if err != nil {
		return nil, err
	}
	rules := make([]cachedApplicationHeaderRule, 0, len(applications))
	for _, application := range applications {
		if application == nil {
			continue
		}
		normalizedRules := application.GetHeaderValidationRules()
		if len(normalizedRules) == 0 {
			continue
		}
		rules = append(rules, cachedApplicationHeaderRule{
			Application: *application,
			Rules:       normalizedRules,
		})
	}
	applicationHeaderRulesCache.rules = cloneCachedApplicationHeaderRules(rules)
	applicationHeaderRulesCache.expiresAt = time.Now().Add(applicationHeaderRulesCacheTTL)
	return rules, nil
}

func cloneCachedApplicationHeaderRules(rules []cachedApplicationHeaderRule) []cachedApplicationHeaderRule {
	if len(rules) == 0 {
		return nil
	}
	clone := make([]cachedApplicationHeaderRule, len(rules))
	for i, rule := range rules {
		clone[i].Application = rule.Application
		clone[i].Rules = append([]types.ApplicationHeaderValidationRule(nil), rule.Rules...)
	}
	return clone
}

// CreateApplication 创建应用
func (s *ApplicationService) CreateApplication(name, description string, status int, sortOrder int, headerRules []types.ApplicationHeaderValidationRule, headerMatchRequired bool) (*model.Application, error) {
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
		AppKey:              appKey,
		Name:                name,
		Description:         description,
		Status:              status,
		SortOrder:           sortOrder,
		HeaderMatchRequired: headerMatchRequired,
	}
	if err := application.SetHeaderValidationRules(headerRules); err != nil {
		return nil, err
	}
	if err := s.ValidateApplicationHeaderRuleConflicts(0, application.Status, application.HeaderMatchRequired, application.GetHeaderValidationRules()); err != nil {
		return nil, err
	}

	if err := application.Insert(); err != nil {
		return nil, err
	}
	InvalidateApplicationHeaderRulesCache()

	return application, nil
}

// UpdateApplication 更新应用
func (s *ApplicationService) UpdateApplication(id int64, name, description string, status int, sortOrder int, headerRules *[]types.ApplicationHeaderValidationRule, headerMatchRequired *bool) error {
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
	if headerMatchRequired != nil {
		application.HeaderMatchRequired = *headerMatchRequired
	}
	if headerRules != nil {
		if err := application.SetHeaderValidationRules(*headerRules); err != nil {
			return err
		}
	}
	if err := s.ValidateApplicationHeaderRuleConflicts(id, application.Status, application.HeaderMatchRequired, application.GetHeaderValidationRules()); err != nil {
		return err
	}

	if err := application.Update(); err != nil {
		return err
	}
	InvalidateApplicationHeaderRulesCache()
	return nil
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

	if err := model.DeleteApplicationByID(id); err != nil {
		return err
	}
	InvalidateApplicationHeaderRulesCache()
	return nil
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
		if err := s.ValidateApplicationHeaderRuleConflicts(id, status, application.HeaderMatchRequired, application.GetHeaderValidationRules()); err != nil {
			return err
		}
	}
	if err := model.UpdateApplicationStatus(id, status); err != nil {
		return err
	}
	InvalidateApplicationHeaderRulesCache()
	return nil
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

// ValidateApplicationHeaderRuleConflicts protects the uniqueness of enabled
// applications that require strict header matching. Non-strict applications may
// overlap with each other, but they cannot overlap with any enabled strict
// application because that would make strict matching ambiguous.
func (s *ApplicationService) ValidateApplicationHeaderRuleConflicts(applicationId int64, status int, headerMatchRequired bool, rules []types.ApplicationHeaderValidationRule) error {
	if status != 1 {
		return nil
	}
	if headerMatchRequired && len(rules) == 0 {
		return fmt.Errorf("%w: strict header matching requires header rules", ErrApplicationHeaderInvalid)
	}
	if len(rules) == 0 {
		return nil
	}
	candidate, err := buildApplicationHeaderRuleConstraints(rules)
	if err != nil {
		return err
	}
	if len(candidate) == 0 {
		if headerMatchRequired {
			return fmt.Errorf("%w: strict header matching requires header rules", ErrApplicationHeaderInvalid)
		}
		return nil
	}

	applications, err := model.GetAllApplications(1)
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
		if !headerMatchRequired && !application.HeaderMatchRequired {
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
	Application           *model.Application
	Applications          []*model.Application
	Matched               bool
	Checked               bool
	Reason                error
	AmbiguousApplications []*model.Application
}

// MatchRequestApplicationByHeaders identifies a request application using the
// configured header rules on enabled applications. If no enabled application
// has header rules, Checked is false.
func (s *ApplicationService) MatchRequestApplicationByHeaders(headers http.Header) (*RequestApplicationMatch, error) {
	rules, err := s.getCachedApplicationHeaderRules()
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return &RequestApplicationMatch{Checked: false}, nil
	}

	matches := make([]*model.Application, 0, 1)
	for _, cachedRule := range rules {
		if types.MatchApplicationHeaderValidationRules(headers, cachedRule.Rules) {
			application := cachedRule.Application
			matches = append(matches, &application)
		}
	}

	if len(matches) == 0 {
		return &RequestApplicationMatch{Checked: true, Reason: ErrApplicationUnrecognized}, nil
	}
	if len(matches) > 1 {
		return &RequestApplicationMatch{
			Application:           matches[0],
			Applications:          matches,
			AmbiguousApplications: matches,
			Matched:               true,
			Checked:               true,
			Reason:                ErrApplicationHeaderAmbiguous,
		}, nil
	}

	return &RequestApplicationMatch{
		Application:  matches[0],
		Applications: matches,
		Matched:      true,
		Checked:      true,
	}, nil
}
