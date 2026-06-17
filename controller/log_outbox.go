package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type replayLogOutboxRequest struct {
	Sink           string   `json:"sink"`
	EventIDs       []string `json:"event_ids"`
	MinLogID       int      `json:"min_log_id"`
	MaxLogID       int      `json:"max_log_id"`
	StartTimestamp int64    `json:"start_timestamp"`
	EndTimestamp   int64    `json:"end_timestamp"`
}

type externalAuditEventRequest struct {
	TargetType      string                 `json:"target_type"`
	TargetID        string                 `json:"target_id"`
	TargetName      string                 `json:"target_name"`
	Action          string                 `json:"action"`
	Result          string                 `json:"result"`
	Summary         string                 `json:"summary"`
	DiffJSON        string                 `json:"diff_json"`
	ApplicationID   int                    `json:"application_id"`
	ApplicationKey  string                 `json:"application_key"`
	ApplicationName string                 `json:"application_name"`
	Other           map[string]interface{} `json:"other"`
}

func GetLogOutboxStats(c *gin.Context) {
	stats, err := model.GetLogOutboxStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}

func ReplayLogOutbox(c *gin.Context) {
	var req replayLogOutboxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if !req.hasSelector() {
		common.ApiErrorMsg(c, "event_ids、log_id 范围或时间范围至少需要提供一个")
		return
	}
	sinks, err := normalizeReplaySinks(req.Sink)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	eventIDs := compactNonEmptyStrings(req.EventIDs)
	rebuilt, err := rebuildMissingLogOutbox(req, eventIDs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for _, sink := range sinks {
		if len(eventIDs) > 0 {
			if err := model.ResetLogOutboxSinkByEventIDs(sink, eventIDs); err != nil {
				common.ApiError(c, err)
				return
			}
			continue
		}
		if req.MinLogID > 0 || req.MaxLogID > 0 {
			if err := model.ResetLogOutboxSinkByLogIDRange(sink, req.MinLogID, req.MaxLogID); err != nil {
				common.ApiError(c, err)
				return
			}
			continue
		}
		if err := model.ResetLogOutboxSinkByTimeRange(sink, req.StartTimestamp, req.EndTimestamp); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	common.ApiSuccess(c, gin.H{
		"sinks":   sinks,
		"rebuilt": rebuilt,
	})
}

func ValidateLogOutboxConsistency(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	stats, err := model.ValidateLogOutboxConsistency(startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}

func RecordExternalAuditEvent(c *gin.Context) {
	var req externalAuditEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(req.Action) == "" {
		common.ApiErrorMsg(c, "action is required")
		return
	}
	if strings.TrimSpace(req.Result) == "" {
		req.Result = "success"
	}
	if strings.TrimSpace(req.Summary) == "" {
		req.Summary = req.Action
	}
	params := auditParamsFromContext(c)
	params.TargetType = req.TargetType
	params.TargetID = req.TargetID
	params.TargetName = req.TargetName
	params.Action = req.Action
	params.Result = req.Result
	params.Summary = req.Summary
	params.DiffJSON = req.DiffJSON
	params.ApplicationID = req.ApplicationID
	params.ApplicationKey = req.ApplicationKey
	params.ApplicationName = req.ApplicationName
	params.Other = req.Other
	if params.ApplicationID == 0 {
		params.ApplicationID = params.RequestContext.Application.Id
	}
	if params.ApplicationKey == "" {
		params.ApplicationKey = params.RequestContext.Application.Key
	}
	if params.ApplicationName == "" {
		params.ApplicationName = params.RequestContext.Application.Name
	}
	if err := model.RecordAuditLog(params); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func (r replayLogOutboxRequest) hasSelector() bool {
	return len(compactNonEmptyStrings(r.EventIDs)) > 0 ||
		r.MinLogID > 0 ||
		r.MaxLogID > 0 ||
		r.StartTimestamp > 0 ||
		r.EndTimestamp > 0
}

func normalizeReplaySinks(raw string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "both", "all":
		return []string{model.LogOutboxSinkKafka, model.LogOutboxSinkClickHouse}, nil
	case model.LogOutboxSinkKafka:
		return []string{model.LogOutboxSinkKafka}, nil
	case model.LogOutboxSinkClickHouse, "click_house":
		return []string{model.LogOutboxSinkClickHouse}, nil
	default:
		return nil, errors.New("sink must be kafka, clickhouse or both")
	}
}

func compactNonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func rebuildMissingLogOutbox(req replayLogOutboxRequest, eventIDs []string) (int64, error) {
	if len(eventIDs) > 0 {
		return model.RebuildMissingLogOutboxByEventIDs(eventIDs)
	}
	if req.MinLogID > 0 || req.MaxLogID > 0 {
		return model.RebuildMissingLogOutboxByLogIDRange(req.MinLogID, req.MaxLogID)
	}
	return model.RebuildMissingLogOutboxByTimeRange(req.StartTimestamp, req.EndTimestamp)
}

func auditParamsFromContext(c *gin.Context) model.RecordAuditLogParams {
	requestContext := model.BuildRequestContextSnapshot(c, nil, nil)
	if requestContext.User.Id == 0 {
		requestContext.User.Id = c.GetInt("id")
	}
	if requestContext.User.Username == "" {
		requestContext.User.Username = c.GetString("username")
	}
	if requestContext.User.Role == 0 {
		requestContext.User.Role = c.GetInt("role")
	}
	if requestContext.Organization.CompanyId == 0 {
		if user, err := model.GetUserCache(requestContext.User.Id); err == nil && user != nil {
			requestContext.Organization.CompanyId = user.CompanyId
			requestContext.Organization.CompanyName = user.CompanyName
			requestContext.Organization.CompanyCode = user.CompanyCode
			requestContext.Organization.DepartmentId = user.DepartmentId
			requestContext.Organization.DepartmentName = user.DepartmentName
			requestContext.Organization.DepartmentPath = user.DepartmentPath
			requestContext.Organization.DepartmentLevel = user.DepartmentLevel
			if user.DepartmentHierarchy != "" {
				_ = common.UnmarshalJsonStr(user.DepartmentHierarchy, &requestContext.Organization.DepartmentHierarchy)
			}
		}
	}
	params := model.RecordAuditLogParams{
		ActorUserID:       requestContext.User.Id,
		ActorUsername:     requestContext.User.Username,
		ActorRole:         requestContext.User.Role,
		ActorCompanyID:    requestContext.Organization.CompanyId,
		ActorDepartmentID: requestContext.Organization.DepartmentId,
		RequestID:         c.GetString(common.RequestIdKey),
		RequestMethod:     requestContext.Request.Method,
		RequestPath:       requestContext.Request.Path,
		ClientIP:          requestContext.Request.ClientIp,
		UserAgent:         requestContext.Request.UserAgent,
		ApplicationID:     requestContext.Application.Id,
		ApplicationKey:    requestContext.Application.Key,
		ApplicationName:   requestContext.Application.Name,
		RequestContext:    requestContext,
	}
	if params.RequestMethod == "" && c.Request != nil {
		params.RequestMethod = c.Request.Method
	}
	if params.RequestPath == "" && c.Request != nil && c.Request.URL != nil {
		params.RequestPath = c.Request.URL.Path
	}
	if params.ClientIP == "" {
		params.ClientIP = c.ClientIP()
	}
	return params
}
