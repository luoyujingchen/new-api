package service

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

var timeFormatRegex = regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)

// ValidateTimeSlot validates a single time slot
func ValidateTimeSlot(slot dto.TimeSlot) error {
	if !timeFormatRegex.MatchString(slot.StartTime) {
		return fmt.Errorf("invalid start_time format: %s, expected HH:MM", slot.StartTime)
	}
	if !timeFormatRegex.MatchString(slot.EndTime) {
		return fmt.Errorf("invalid end_time format: %s, expected HH:MM", slot.EndTime)
	}
	for _, w := range slot.Weekdays {
		if w < 0 || w > 6 {
			return fmt.Errorf("invalid weekday: %d, must be 0-6", w)
		}
	}
	return nil
}

// ValidateCreateRateLimitRequest validates the create request
func ValidateCreateRateLimitRequest(req *dto.CreateRateLimitRequest) error {
	if len(req.TimeSlots) != len(req.Rpms) {
		return errors.New("time_slots and rpms must have the same length")
	}
	for _, slot := range req.TimeSlots {
		if err := ValidateTimeSlot(slot); err != nil {
			return err
		}
	}
	for _, rpm := range req.Rpms {
		if rpm < 0 {
			return errors.New("rpm must be >= 0")
		}
	}
	return nil
}

// ValidateUpdateRateLimitRequest validates the update request
func ValidateUpdateRateLimitRequest(req *dto.UpdateRateLimitRequest) error {
	if len(req.TimeSlots) != len(req.Rpms) {
		return errors.New("time_slots and rpms must have the same length")
	}
	for _, slot := range req.TimeSlots {
		if err := ValidateTimeSlot(slot); err != nil {
			return err
		}
	}
	for _, rpm := range req.Rpms {
		if rpm < 0 {
			return errors.New("rpm must be >= 0")
		}
	}
	return nil
}

// CreateRateLimitService creates a new rate limit rule
func CreateRateLimitService(req *dto.CreateRateLimitRequest) (*dto.RateLimitResponse, error) {
	// Validate org exists
	if err := validateOrgExists(req.OrgType, req.OrgId); err != nil {
		return nil, err
	}

	// Validate and resolve model
	resolvedModelId, resolvedModelName, err := model.ValidateModelConsistency(req.ModelId, req.ModelName)
	if err != nil {
		return nil, err
	}

	// Validate request
	if err := ValidateCreateRateLimitRequest(req); err != nil {
		return nil, err
	}

	// Serialize time_slots and rpms
	timeSlotsJSON, err := common.Marshal(req.TimeSlots)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize time_slots: %w", err)
	}
	rpmsJSON, err := common.Marshal(req.Rpms)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize rpms: %w", err)
	}

	rule := &model.OrganizationRateLimit{
		OrgType:   req.OrgType,
		OrgId:     req.OrgId,
		ModelId:   resolvedModelId,
		ModelName: resolvedModelName,
		TimeSlots: string(timeSlotsJSON),
		Rpms:      string(rpmsJSON),
		Priority:  req.Priority,
		Status:    req.Status,
	}

	if err := model.CreateRateLimit(rule); err != nil {
		return nil, err
	}

	// Invalidate cache
	InvalidateRateLimitCache(req.OrgType, req.OrgId)

	return buildRateLimitResponse(rule)
}

// UpdateRateLimitService updates an existing rate limit rule
func UpdateRateLimitService(id int, req *dto.UpdateRateLimitRequest) (*dto.RateLimitResponse, error) {
	rule, err := model.GetRateLimitById(id)
	if err != nil {
		return nil, err
	}

	// Validate request
	if err := ValidateUpdateRateLimitRequest(req); err != nil {
		return nil, err
	}

	// Serialize time_slots and rpms
	timeSlotsJSON, err := common.Marshal(req.TimeSlots)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize time_slots: %w", err)
	}
	rpmsJSON, err := common.Marshal(req.Rpms)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize rpms: %w", err)
	}

	rule.TimeSlots = string(timeSlotsJSON)
	rule.Rpms = string(rpmsJSON)
	rule.Priority = req.Priority
	rule.Status = req.Status

	if err := model.UpdateRateLimit(rule); err != nil {
		return nil, err
	}

	// Invalidate cache
	InvalidateRateLimitCache(rule.OrgType, rule.OrgId)

	return buildRateLimitResponse(rule)
}

// DeleteRateLimitService deletes a rate limit rule
func DeleteRateLimitService(id int) error {
	rule, err := model.GetRateLimitById(id)
	if err != nil {
		return err
	}

	if err := model.DeleteRateLimitById(id); err != nil {
		return err
	}

	// Invalidate cache
	InvalidateRateLimitCache(rule.OrgType, rule.OrgId)
	return nil
}

// GetRateLimitService returns a single rate limit rule
func GetRateLimitService(id int) (*dto.RateLimitResponse, error) {
	rule, err := model.GetRateLimitById(id)
	if err != nil {
		return nil, err
	}
	return buildRateLimitResponse(rule)
}

// ListRateLimitsService returns paginated rate limit rules for an org
func ListRateLimitsService(query *dto.RateLimitListQuery, pageInfo *common.PageInfo) ([]*dto.RateLimitResponse, int64, error) {
	rules, total, err := model.GetRateLimitsByOrg(query.OrgType, query.OrgId, query.ModelId, query.Status, pageInfo)
	if err != nil {
		return nil, 0, err
	}

	items := make([]*dto.RateLimitResponse, 0, len(rules))
	for _, rule := range rules {
		resp, err := buildRateLimitResponse(rule)
		if err != nil {
			continue
		}
		items = append(items, resp)
	}

	return items, total, nil
}

// GetUserEffectiveRateLimit returns the effective rate limit for a user
func GetUserEffectiveRateLimit(userId int, modelName string, modelId int) (*dto.UserRateLimitResponse, error) {
	companyId, departmentId, err := model.GetUserCompanyAndDepartment(userId)
	if err != nil {
		return nil, err
	}

	if companyId == 0 && departmentId == 0 {
		return &dto.UserRateLimitResponse{Source: "none"}, nil
	}

	now := time.Now()
	weekday := int(now.Weekday()) // 0=Sunday
	currentTime := now.Format("15:04")

	// Try department chain first (current dept -> parent -> ancestors)
	if departmentId > 0 {
		result := matchDepartmentChain(departmentId, companyId, modelName, weekday, currentTime)
		if result != nil {
			return result, nil
		}
	}

	// Try company rules
	if companyId > 0 {
		result := matchOrgRules("company", companyId, modelName, weekday, currentTime)
		if result != nil {
			return result, nil
		}
	}

	return &dto.UserRateLimitResponse{Source: "none"}, nil
}

// matchDepartmentChain tries to match rules starting from the current department up through ancestors
func matchDepartmentChain(departmentId int, companyId int, modelName string, weekday int, currentTime string) *dto.UserRateLimitResponse {
	// Get ancestor chain: [root, ..., parent]
	ancestors, err := model.GetDepartmentAncestorIds(departmentId)
	if err != nil {
		ancestors = nil
	}

	// Build the chain: [current, parent, grandparent, ..., root]
	chain := []int{departmentId}
	if len(ancestors) > 0 {
		// ancestors are in root-first order, reverse to get parent-first
		for i := len(ancestors) - 1; i >= 0; i-- {
			chain = append(chain, ancestors[i])
		}
	}

	for _, deptId := range chain {
		result := matchOrgRules("department", deptId, modelName, weekday, currentTime)
		if result != nil {
			return result
		}
	}

	return nil
}

// MatchOrgRateLimitResult holds the result of matching organization rate limits (used by middleware)
type MatchOrgRateLimitResult struct {
	Matched   bool
	OrgType   string
	OrgId     int
	Rpm       int
	ModelName string
}

// MatchOrgRateLimitForMiddleware is the main entry point for the middleware to check org rate limits
func MatchOrgRateLimitForMiddleware(userId int, modelName string) *MatchOrgRateLimitResult {
	companyId, departmentId, err := model.GetUserCompanyAndDepartment(userId)
	if err != nil || (companyId == 0 && departmentId == 0) {
		return nil
	}

	now := time.Now()
	weekday := int(now.Weekday())
	currentTime := now.Format("15:04")

	// Try department chain first
	if departmentId > 0 {
		result := matchOrgRateLimitChain(departmentId, "department", modelName, weekday, currentTime)
		if result != nil {
			return result
		}
	}

	// Try company rules
	if companyId > 0 {
		result := matchOrgRateLimitSingle("company", companyId, modelName, weekday, currentTime)
		if result != nil {
			return result
		}
	}

	return nil
}

// matchOrgRateLimitChain tries to match rules for a department and its ancestors
func matchOrgRateLimitChain(departmentId int, orgType string, modelName string, weekday int, currentTime string) *MatchOrgRateLimitResult {
	// Try current department first
	result := matchOrgRateLimitSingle(orgType, departmentId, modelName, weekday, currentTime)
	if result != nil {
		return result
	}

	// Try ancestors
	ancestors, err := model.GetDepartmentAncestorIds(departmentId)
	if err != nil {
		return nil
	}

	// ancestors are in root-first order, try from parent-first (reverse)
	for i := len(ancestors) - 1; i >= 0; i-- {
		result := matchOrgRateLimitSingle(orgType, ancestors[i], modelName, weekday, currentTime)
		if result != nil {
			return result
		}
	}

	return nil
}

// matchOrgRateLimitSingle tries to match rules for a single org
func matchOrgRateLimitSingle(orgType string, orgId int, modelName string, weekday int, currentTime string) *MatchOrgRateLimitResult {
	rules, err := GetCachedRateLimits(orgType, orgId)
	if err != nil || len(rules) == 0 {
		return nil
	}

	// First pass: try exact model match
	result := matchRulesWithModelFilter(rules, modelName, weekday, currentTime, true)
	if result != nil {
		return result
	}

	// Second pass: try wildcard (model_name is empty) rules
	result = matchRulesWithModelFilter(rules, modelName, weekday, currentTime, false)
	return result
}

// matchRulesWithModelFilter matches rules filtering by model specificity
func matchRulesWithModelFilter(rules []*model.OrganizationRateLimit, modelName string, weekday int, currentTime string, exactModel bool) *MatchOrgRateLimitResult {
	for _, rule := range rules {
		// Filter by model specificity
		if exactModel {
			// Only consider rules with a specific model name that matches
			if rule.ModelName == "" || rule.ModelName != modelName {
				continue
			}
		} else {
			// Only consider wildcard rules
			if rule.ModelName != "" {
				continue
			}
		}

		// Try to match time slots
		rpm := matchTimeSlots(rule, weekday, currentTime)
		if rpm >= 0 {
			return &MatchOrgRateLimitResult{
				Matched:   true,
				OrgType:   rule.OrgType,
				OrgId:     rule.OrgId,
				Rpm:       rpm,
				ModelName: rule.ModelName,
			}
		}
	}
	return nil
}

// matchTimeSlots checks if any time slot in the rule matches the current time
// Returns the matched RPM, or -1 if no match
func matchTimeSlots(rule *model.OrganizationRateLimit, weekday int, currentTime string) int {
	var slots []dto.TimeSlot
	if err := common.UnmarshalJsonStr(rule.TimeSlots, &slots); err != nil {
		return -1
	}

	var rpms []int
	if err := common.UnmarshalJsonStr(rule.Rpms, &rpms); err != nil {
		return -1
	}

	for i, slot := range slots {
		if i >= len(rpms) {
			break
		}
		rpm := rpms[i]

		// RPM 0 means this slot should not count as a match
		if rpm == 0 {
			continue
		}

		// Check weekday
		if !weekdayMatches(slot.Weekdays, weekday) {
			continue
		}

		// Check time range
		if timeInRange(slot.StartTime, slot.EndTime, currentTime) {
			return rpm
		}
	}

	return -1
}

// weekdayMatches checks if the current weekday is in the allowed list
// Empty weekdays means every day
func weekdayMatches(weekdays []int, currentWeekday int) bool {
	if len(weekdays) == 0 {
		return true
	}
	for _, w := range weekdays {
		if w == currentWeekday {
			return true
		}
	}
	return false
}

// timeInRange checks if currentTime is within the range [startTime, endTime]
// Supports cross-day ranges where startTime > endTime (e.g. 23:00-02:00)
func timeInRange(startTime, endTime, currentTime string) bool {
	if startTime == endTime {
		// Zero-duration range, only matches exact time
		return currentTime == startTime
	}

	if startTime < endTime {
		// Normal range: e.g., 09:00-18:00
		return currentTime >= startTime && currentTime < endTime
	}

	// Cross-day range: e.g., 23:00-02:00
	// Current time is in range if it's >= startTime OR < endTime
	return currentTime >= startTime || currentTime < endTime
}

// matchOrgRules matches rules for a specific org (for the user query API)
func matchOrgRules(orgType string, orgId int, modelName string, weekday int, currentTime string) *dto.UserRateLimitResponse {
	rules, err := GetCachedRateLimits(orgType, orgId)
	if err != nil || len(rules) == 0 {
		return nil
	}

	// First pass: exact model match
	result := matchRulesForQuery(rules, orgType, orgId, modelName, weekday, currentTime, true)
	if result != nil {
		return result
	}

	// Second pass: wildcard
	result = matchRulesForQuery(rules, orgType, orgId, modelName, weekday, currentTime, false)
	return result
}

func matchRulesForQuery(rules []*model.OrganizationRateLimit, orgType string, orgId int, modelName string, weekday int, currentTime string, exactModel bool) *dto.UserRateLimitResponse {
	for _, rule := range rules {
		if exactModel {
			if rule.ModelName == "" || rule.ModelName != modelName {
				continue
			}
		} else {
			if rule.ModelName != "" {
				continue
			}
		}

		var slots []dto.TimeSlot
		if err := common.UnmarshalJsonStr(rule.TimeSlots, &slots); err != nil {
			continue
		}
		var rpms []int
		if err := common.UnmarshalJsonStr(rule.Rpms, &rpms); err != nil {
			continue
		}

		for i, slot := range slots {
			if i >= len(rpms) {
				break
			}
			rpm := rpms[i]
			if rpm == 0 {
				continue
			}
			if !weekdayMatches(slot.Weekdays, weekday) {
				continue
			}
			if timeInRange(slot.StartTime, slot.EndTime, currentTime) {
				orgName := getOrgName(orgType, orgId)
				return &dto.UserRateLimitResponse{
					Source:    orgType,
					OrgName:   orgName,
					OrgType:   orgType,
					OrgId:     orgId,
					TimeSlot:  &slot,
					Rpm:       rpm,
					Weekday:   weekday,
					ModelId:   rule.ModelId,
					ModelName: rule.ModelName,
					Priority:  rule.Priority,
					RuleId:    rule.Id,
				}
			}
		}
	}
	return nil
}

func getOrgName(orgType string, orgId int) string {
	switch orgType {
	case "company":
		company, err := model.GetCompanyById(orgId)
		if err != nil {
			return ""
		}
		return company.Name
	case "department":
		dept, err := model.GetDepartmentById(orgId)
		if err != nil {
			return ""
		}
		return dept.Name
	}
	return ""
}

func validateOrgExists(orgType string, orgId int) error {
	switch orgType {
	case "company":
		_, err := model.GetCompanyById(orgId)
		if err != nil {
			return fmt.Errorf("company not found: %w", err)
		}
	case "department":
		_, err := model.GetDepartmentById(orgId)
		if err != nil {
			return fmt.Errorf("department not found: %w", err)
		}
	default:
		return fmt.Errorf("invalid org_type: %s", orgType)
	}
	return nil
}

func buildRateLimitResponse(rule *model.OrganizationRateLimit) (*dto.RateLimitResponse, error) {
	var slots []dto.TimeSlot
	if err := common.UnmarshalJsonStr(rule.TimeSlots, &slots); err != nil {
		return nil, err
	}

	var rpms []int
	if err := common.UnmarshalJsonStr(rule.Rpms, &rpms); err != nil {
		return nil, err
	}

	orgName := getOrgName(rule.OrgType, rule.OrgId)

	modelNameDisplay := rule.ModelName
	if modelNameDisplay == "" {
		modelNameDisplay = "All Models"
	}

	return &dto.RateLimitResponse{
		Id:        rule.Id,
		OrgType:   rule.OrgType,
		OrgId:     rule.OrgId,
		OrgName:   orgName,
		ModelId:   rule.ModelId,
		ModelName: modelNameDisplay,
		TimeSlots: slots,
		Rpms:      rpms,
		Priority:  rule.Priority,
		Status:    rule.Status,
		CreatedAt: rule.CreatedAt,
		UpdatedAt: rule.UpdatedAt,
	}, nil
}

// ParseTimeString parses "HH:MM" into hour and minute
func ParseTimeString(timeStr string) (int, int, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format: %s", timeStr)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return hour, minute, nil
}
