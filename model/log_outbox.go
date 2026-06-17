package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

const (
	LogOutboxEventTypeUsage = "usage"
	LogOutboxEventTypeAudit = "audit"

	LogOutboxSinkKafka      = "kafka"
	LogOutboxSinkClickHouse = "clickhouse"

	LogOutboxStatusPending    = "pending"
	LogOutboxStatusProcessing = "processing"
	LogOutboxStatusSent       = "sent"
	LogOutboxStatusFailed     = "failed"
	LogOutboxStatusDisabled   = "disabled"
)

type LogOutbox struct {
	Id                    int64                `json:"id" gorm:"primaryKey"`
	EventID               string               `json:"event_id" gorm:"type:varchar(128);uniqueIndex;not null"`
	EventType             string               `json:"event_type" gorm:"type:varchar(32);index;not null"`
	SourceLogID           int                  `json:"source_log_id" gorm:"index;not null"`
	SourceRequestID       string               `json:"source_request_id" gorm:"type:varchar(128);index;default:''"`
	PayloadJSON           LogOutboxPayloadJSON `json:"payload_json" gorm:"not null"`
	UsageQuota            int                  `json:"usage_quota" gorm:"default:0"`
	UsagePromptTokens     int                  `json:"usage_prompt_tokens" gorm:"default:0"`
	UsageCompletionTokens int                  `json:"usage_completion_tokens" gorm:"default:0"`
	UsageTotalTokens      int                  `json:"usage_total_tokens" gorm:"default:0"`
	PayloadSizeBytes      int                  `json:"payload_size_bytes" gorm:"default:0"`
	PayloadTruncated      bool                 `json:"payload_truncated" gorm:"default:false"`
	KafkaStatus           string               `json:"kafka_status" gorm:"type:varchar(16);index;default:'pending'"`
	KafkaRetryCount       int                  `json:"kafka_retry_count" gorm:"default:0"`
	KafkaNextRetryAt      int64                `json:"kafka_next_retry_at" gorm:"bigint;index;default:0"`
	KafkaLastError        string               `json:"kafka_last_error" gorm:"type:text"`
	ClickHouseStatus      string               `json:"clickhouse_status" gorm:"column:clickhouse_status;type:varchar(16);index;default:'pending'"`
	ClickHouseRetryCount  int                  `json:"clickhouse_retry_count" gorm:"column:clickhouse_retry_count;default:0"`
	ClickHouseNextRetryAt int64                `json:"clickhouse_next_retry_at" gorm:"column:clickhouse_next_retry_at;bigint;index;default:0"`
	ClickHouseLastError   string               `json:"clickhouse_last_error" gorm:"column:clickhouse_last_error;type:text"`
	CreatedAt             int64                `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt             int64                `json:"updated_at" gorm:"autoUpdateTime"`
}

type LogOutboxPayloadJSON string

func (LogOutboxPayloadJSON) GormDataType() string {
	return "text"
}

func (LogOutboxPayloadJSON) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "mysql" {
		return "LONGTEXT"
	}
	return "TEXT"
}

type CanonicalLogEvent struct {
	Version   int            `json:"version"`
	EventID   string         `json:"event_id"`
	EventType string         `json:"event_type"`
	Usage     *UsageLogEvent `json:"usage,omitempty"`
	Audit     *AuditLogEvent `json:"audit,omitempty"`
}

type UsageLogEvent struct {
	EventID           string                 `json:"event_id"`
	LogID             int                    `json:"log_id"`
	UserID            int                    `json:"user_id"`
	Username          string                 `json:"username"`
	CreatedAt         int64                  `json:"created_at"`
	Type              int                    `json:"type"`
	Content           string                 `json:"content"`
	TokenID           int                    `json:"token_id"`
	TokenName         string                 `json:"token_name"`
	ModelName         string                 `json:"model_name"`
	ChannelID         int                    `json:"channel_id"`
	ChannelName       string                 `json:"channel_name,omitempty"`
	Quota             int                    `json:"quota"`
	PromptTokens      int                    `json:"prompt_tokens"`
	CompletionTokens  int                    `json:"completion_tokens"`
	UseTime           int                    `json:"use_time"`
	IsStream          bool                   `json:"is_stream"`
	Group             string                 `json:"group"`
	IP                string                 `json:"ip,omitempty"`
	RequestID         string                 `json:"request_id,omitempty"`
	UpstreamRequestID string                 `json:"upstream_request_id,omitempty"`
	Other             map[string]interface{} `json:"other,omitempty"`
	RequestContext    RequestContextSnapshot `json:"request_context"`
	Payload           *UsageLogPayload       `json:"payload,omitempty"`
}

type AuditLogEvent struct {
	EventID           string                 `json:"event_id"`
	LogID             int                    `json:"log_id"`
	CreatedAt         int64                  `json:"created_at"`
	ActorUserID       int                    `json:"actor_user_id"`
	ActorUsername     string                 `json:"actor_username,omitempty"`
	ActorRole         int                    `json:"actor_role,omitempty"`
	ActorCompanyID    int64                  `json:"actor_company_id,omitempty"`
	ActorDepartmentID int64                  `json:"actor_department_id,omitempty"`
	TargetType        string                 `json:"target_type,omitempty"`
	TargetID          string                 `json:"target_id,omitempty"`
	TargetName        string                 `json:"target_name,omitempty"`
	Action            string                 `json:"action"`
	Result            string                 `json:"result"`
	Summary           string                 `json:"summary"`
	DiffJSON          string                 `json:"diff_json,omitempty"`
	RequestID         string                 `json:"request_id,omitempty"`
	RequestMethod     string                 `json:"request_method,omitempty"`
	RequestPath       string                 `json:"request_path,omitempty"`
	ClientIP          string                 `json:"client_ip,omitempty"`
	UserAgent         string                 `json:"user_agent,omitempty"`
	ApplicationID     int                    `json:"application_id,omitempty"`
	ApplicationKey    string                 `json:"application_key,omitempty"`
	ApplicationName   string                 `json:"application_name,omitempty"`
	Other             map[string]interface{} `json:"other,omitempty"`
	RequestContext    RequestContextSnapshot `json:"request_context"`
}

type RecordAuditLogParams struct {
	ActorUserID       int
	ActorUsername     string
	ActorRole         int
	ActorCompanyID    int64
	ActorDepartmentID int64
	TargetType        string
	TargetID          string
	TargetName        string
	Action            string
	Result            string
	Summary           string
	DiffJSON          string
	RequestID         string
	RequestMethod     string
	RequestPath       string
	ClientIP          string
	UserAgent         string
	ApplicationID     int
	ApplicationKey    string
	ApplicationName   string
	RequestContext    RequestContextSnapshot
	Other             map[string]interface{}
}

func EnqueueUsageLogOutbox(log *Log, payload *UsageLogPayload) error {
	if log == nil || log.Id == 0 || LOG_DB == nil {
		return nil
	}
	eventID := stableLogEventID(LogOutboxEventTypeUsage, log.Id)
	event, err := BuildUsageLogEvent(eventID, log, payload)
	if err != nil {
		return err
	}
	return enqueueCanonicalLogEvent(eventID, LogOutboxEventTypeUsage, log, CanonicalLogEvent{
		Version:   1,
		EventID:   eventID,
		EventType: LogOutboxEventTypeUsage,
		Usage:     event,
	})
}

func EnqueueAuditLogOutbox(log *Log) error {
	if log == nil || log.Id == 0 || LOG_DB == nil {
		return nil
	}
	eventID := stableLogEventID(LogOutboxEventTypeAudit, log.Id)
	event, err := BuildAuditLogEvent(eventID, log)
	if err != nil {
		return err
	}
	return enqueueCanonicalLogEvent(eventID, LogOutboxEventTypeAudit, log, CanonicalLogEvent{
		Version:   1,
		EventID:   eventID,
		EventType: LogOutboxEventTypeAudit,
		Audit:     event,
	})
}

func RecordAuditLog(params RecordAuditLogParams) error {
	if LOG_DB == nil {
		return nil
	}
	other := params.Other
	if other == nil {
		other = make(map[string]interface{})
	}
	audit := map[string]interface{}{
		"actor_user_id":       params.ActorUserID,
		"actor_username":      params.ActorUsername,
		"actor_role":          params.ActorRole,
		"actor_company_id":    params.ActorCompanyID,
		"actor_department_id": params.ActorDepartmentID,
		"target_type":         params.TargetType,
		"target_id":           params.TargetID,
		"target_name":         params.TargetName,
		"action":              params.Action,
		"result":              params.Result,
		"summary":             params.Summary,
		"diff_json":           params.DiffJSON,
		"request_method":      params.RequestMethod,
		"request_path":        params.RequestPath,
		"client_ip":           params.ClientIP,
		"user_agent":          params.UserAgent,
		"application_id":      params.ApplicationID,
		"application_key":     params.ApplicationKey,
		"application_name":    params.ApplicationName,
	}
	other["audit"] = audit
	if params.RequestContext.User.Id != 0 {
		other["request_context"] = params.RequestContext
	}
	log := &Log{
		UserId:    params.ActorUserID,
		Username:  params.ActorUsername,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeManage,
		Content:   params.Summary,
		Ip:        params.ClientIP,
		RequestId: params.RequestID,
		Other:     jsonString(other),
	}
	if log.Content == "" {
		log.Content = params.Action
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		return err
	}
	return EnqueueAuditLogOutbox(log)
}

func BuildUsageLogEvent(eventID string, log *Log, payload *UsageLogPayload) (*UsageLogEvent, error) {
	if log == nil {
		return nil, fmt.Errorf("log is nil")
	}
	other := parseLogOther(log.Other)
	requestContext := requestContextFromOther(other)
	if requestContext.User.Id == 0 {
		requestContext = BuildRequestContextSnapshot(nil, log, other)
	}
	event := &UsageLogEvent{
		EventID:           eventID,
		LogID:             log.Id,
		UserID:            log.UserId,
		Username:          log.Username,
		CreatedAt:         log.CreatedAt,
		Type:              log.Type,
		Content:           log.Content,
		TokenID:           log.TokenId,
		TokenName:         log.TokenName,
		ModelName:         log.ModelName,
		ChannelID:         log.ChannelId,
		ChannelName:       log.ChannelName,
		Quota:             log.Quota,
		PromptTokens:      log.PromptTokens,
		CompletionTokens:  log.CompletionTokens,
		UseTime:           log.UseTime,
		IsStream:          log.IsStream,
		Group:             log.Group,
		IP:                log.Ip,
		RequestID:         log.RequestId,
		UpstreamRequestID: log.UpstreamRequestId,
		Other:             other,
		RequestContext:    requestContext,
	}
	if payload != nil {
		sanitized := sanitizeUsageLogPayload(*payload)
		event.Payload = &sanitized
	}
	return event, nil
}

func BuildAuditLogEvent(eventID string, log *Log) (*AuditLogEvent, error) {
	if log == nil {
		return nil, fmt.Errorf("log is nil")
	}
	other := parseLogOther(log.Other)
	requestContext := requestContextFromOther(other)
	if requestContext.User.Id == 0 {
		requestContext = BuildRequestContextSnapshot(nil, log, other)
	}
	diffJSON := ""
	if adminInfo, ok := other["admin_info"]; ok {
		if bytes, err := common.Marshal(adminInfo); err == nil {
			diffJSON = string(bytes)
		}
	}
	event := &AuditLogEvent{
		EventID:           eventID,
		LogID:             log.Id,
		CreatedAt:         log.CreatedAt,
		ActorUserID:       log.UserId,
		ActorUsername:     log.Username,
		ActorRole:         requestContext.User.Role,
		ActorCompanyID:    requestContext.Organization.CompanyId,
		ActorDepartmentID: requestContext.Organization.DepartmentId,
		Action:            inferAuditAction(log),
		Result:            "success",
		Summary:           log.Content,
		DiffJSON:          diffJSON,
		RequestID:         log.RequestId,
		RequestMethod:     requestContext.Request.Method,
		RequestPath:       requestContext.Request.Path,
		ClientIP:          log.Ip,
		UserAgent:         requestContext.Request.UserAgent,
		ApplicationID:     requestContext.Application.Id,
		ApplicationKey:    requestContext.Application.Key,
		ApplicationName:   requestContext.Application.Name,
		Other:             other,
		RequestContext:    requestContext,
	}
	if event.ClientIP == "" {
		event.ClientIP = requestContext.Request.ClientIp
	}
	applyAuditFields(event, other)
	return event, nil
}

func GetLogOutboxByEventID(eventID string) (*LogOutbox, error) {
	var outbox LogOutbox
	if err := LOG_DB.Where("event_id = ?", eventID).First(&outbox).Error; err != nil {
		return nil, err
	}
	return &outbox, nil
}

type LogOutboxStats struct {
	PendingKafka            int64 `json:"pending_kafka"`
	PendingClickHouse       int64 `json:"pending_clickhouse"`
	FailedKafka             int64 `json:"failed_kafka"`
	FailedClickHouse        int64 `json:"failed_clickhouse"`
	MaxKafkaLagSeconds      int64 `json:"max_kafka_lag_seconds"`
	MaxClickHouseLagSeconds int64 `json:"max_clickhouse_lag_seconds"`
	PayloadTruncatedCount   int64 `json:"payload_truncated_count"`
	PayloadCapturedBytes    int64 `json:"payload_captured_bytes"`
}

func GetLogOutboxStats() (LogOutboxStats, error) {
	stats := LogOutboxStats{}
	if LOG_DB == nil {
		return stats, nil
	}
	now := common.GetTimestamp()
	if err := LOG_DB.Model(&LogOutbox{}).Where("kafka_status IN ?", []string{LogOutboxStatusPending, LogOutboxStatusFailed, LogOutboxStatusProcessing}).Count(&stats.PendingKafka).Error; err != nil {
		return stats, err
	}
	if err := LOG_DB.Model(&LogOutbox{}).Where("clickhouse_status IN ?", []string{LogOutboxStatusPending, LogOutboxStatusFailed, LogOutboxStatusProcessing}).Count(&stats.PendingClickHouse).Error; err != nil {
		return stats, err
	}
	if err := LOG_DB.Model(&LogOutbox{}).Where("kafka_status = ?", LogOutboxStatusFailed).Count(&stats.FailedKafka).Error; err != nil {
		return stats, err
	}
	if err := LOG_DB.Model(&LogOutbox{}).Where("clickhouse_status = ?", LogOutboxStatusFailed).Count(&stats.FailedClickHouse).Error; err != nil {
		return stats, err
	}
	stats.MaxKafkaLagSeconds = maxOutboxLagSeconds("kafka_status", now)
	stats.MaxClickHouseLagSeconds = maxOutboxLagSeconds("clickhouse_status", now)
	if err := LOG_DB.Model(&LogOutbox{}).Where("event_type = ? AND payload_truncated = ?", LogOutboxEventTypeUsage, true).Count(&stats.PayloadTruncatedCount).Error; err != nil {
		return stats, err
	}
	if err := LOG_DB.Model(&LogOutbox{}).Where("event_type = ?", LogOutboxEventTypeUsage).
		Select("COALESCE(SUM(payload_size_bytes), 0)").Scan(&stats.PayloadCapturedBytes).Error; err != nil {
		return stats, err
	}
	return stats, nil
}

type LogConsistencyStats struct {
	OLTPCount        int64 `json:"oltp_count"`
	OutboxUsageCount int64 `json:"outbox_usage_count"`
	OLTPQuotaSum     int64 `json:"oltp_quota_sum"`
	OutboxQuotaSum   int64 `json:"outbox_quota_sum"`
	OLTPTokensSum    int64 `json:"oltp_tokens_sum"`
	OutboxTokensSum  int64 `json:"outbox_tokens_sum"`
	CountMatched     bool  `json:"count_matched"`
	QuotaMatched     bool  `json:"quota_matched"`
	TokensMatched    bool  `json:"tokens_matched"`
}

func ValidateLogOutboxConsistency(startTimestamp int64, endTimestamp int64) (LogConsistencyStats, error) {
	stats := LogConsistencyStats{}
	usageLogTypes := []int{LogTypeConsume, LogTypeError, LogTypeRefund}
	logTx := LOG_DB.Model(&Log{}).Where("type IN ?", usageLogTypes)
	outboxTx := LOG_DB.Model(&LogOutbox{}).Where("event_type = ?", LogOutboxEventTypeUsage)
	if startTimestamp > 0 {
		logTx = logTx.Where("created_at >= ?", startTimestamp)
		outboxTx = outboxTx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		logTx = logTx.Where("created_at <= ?", endTimestamp)
		outboxTx = outboxTx.Where("created_at <= ?", endTimestamp)
	}
	type aggregateRow struct {
		CountValue int64 `gorm:"column:count_value"`
		QuotaSum   int64 `gorm:"column:quota_sum"`
		TokensSum  int64 `gorm:"column:tokens_sum"`
	}
	var logAgg aggregateRow
	if err := logTx.Select("COUNT(*) AS count_value, COALESCE(SUM(quota), 0) AS quota_sum, COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens_sum").Scan(&logAgg).Error; err != nil {
		return stats, err
	}
	var outboxAgg aggregateRow
	if err := outboxTx.Select("COUNT(*) AS count_value, COALESCE(SUM(usage_quota), 0) AS quota_sum, COALESCE(SUM(usage_total_tokens), 0) AS tokens_sum").Scan(&outboxAgg).Error; err != nil {
		return stats, err
	}
	stats.OLTPCount = logAgg.CountValue
	stats.OLTPQuotaSum = logAgg.QuotaSum
	stats.OLTPTokensSum = logAgg.TokensSum
	stats.OutboxUsageCount = outboxAgg.CountValue
	stats.OutboxQuotaSum = outboxAgg.QuotaSum
	stats.OutboxTokensSum = outboxAgg.TokensSum
	stats.CountMatched = stats.OLTPCount == stats.OutboxUsageCount
	stats.QuotaMatched = stats.OLTPQuotaSum == stats.OutboxQuotaSum
	stats.TokensMatched = stats.OLTPTokensSum == stats.OutboxTokensSum
	return stats, nil
}

func ListLogOutboxDue(sink string, now int64, limit int) ([]LogOutbox, error) {
	if LOG_DB == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	statusColumn, nextRetryColumn, err := logOutboxSinkColumns(sink)
	if err != nil {
		return nil, err
	}
	var events []LogOutbox
	err = LOG_DB.Where(statusColumn+" IN ? AND "+nextRetryColumn+" <= ?", []string{LogOutboxStatusPending, LogOutboxStatusFailed, LogOutboxStatusProcessing}, now).
		Order("id asc").
		Limit(limit).
		Find(&events).Error
	return events, err
}

func ClaimLogOutboxDue(sink string, now int64, limit int, leaseSeconds int64) ([]LogOutbox, error) {
	events, err := ListLogOutboxDue(sink, now, limit)
	if err != nil || len(events) == 0 {
		return events, err
	}
	statusColumn, _, err := logOutboxSinkColumns(sink)
	if err != nil {
		return nil, err
	}
	_, _, nextRetryColumn, errorColumn, err := logOutboxSinkUpdateColumns(sink)
	if err != nil {
		return nil, err
	}
	if leaseSeconds <= 0 {
		leaseSeconds = 300
	}
	claimed := make([]LogOutbox, 0, len(events))
	claimUntil := now + leaseSeconds
	for _, event := range events {
		result := LOG_DB.Model(&LogOutbox{}).
			Where("id = ? AND "+statusColumn+" IN ?", event.Id, []string{LogOutboxStatusPending, LogOutboxStatusFailed, LogOutboxStatusProcessing}).
			Where(nextRetryColumn+" <= ?", now).
			Updates(map[string]interface{}{
				statusColumn:    LogOutboxStatusProcessing,
				nextRetryColumn: claimUntil,
				errorColumn:     "",
			})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		event = refreshClaimedSinkStatus(event, sink, claimUntil)
		claimed = append(claimed, event)
	}
	return claimed, nil
}

func MarkLogOutboxSinkSent(id int64, sink string) error {
	statusColumn, retryColumn, nextRetryColumn, errorColumn, err := logOutboxSinkUpdateColumns(sink)
	if err != nil {
		return err
	}
	return LOG_DB.Model(&LogOutbox{}).Where("id = ?", id).Updates(map[string]interface{}{
		statusColumn:    LogOutboxStatusSent,
		retryColumn:     0,
		nextRetryColumn: 0,
		errorColumn:     "",
	}).Error
}

func MarkLogOutboxSinkDisabled(id int64, sink string) error {
	statusColumn, _, nextRetryColumn, errorColumn, err := logOutboxSinkUpdateColumns(sink)
	if err != nil {
		return err
	}
	return LOG_DB.Model(&LogOutbox{}).Where("id = ?", id).Updates(map[string]interface{}{
		statusColumn:    LogOutboxStatusDisabled,
		nextRetryColumn: 0,
		errorColumn:     "",
	}).Error
}

func MarkLogOutboxSinkFailed(id int64, sink string, retryCount int, nextRetryAt int64, lastError string) error {
	statusColumn, retryColumn, nextRetryColumn, errorColumn, err := logOutboxSinkUpdateColumns(sink)
	if err != nil {
		return err
	}
	if len(lastError) > 2000 {
		lastError = lastError[:2000]
	}
	return LOG_DB.Model(&LogOutbox{}).Where("id = ?", id).Updates(map[string]interface{}{
		statusColumn:    LogOutboxStatusFailed,
		retryColumn:     retryCount,
		nextRetryColumn: nextRetryAt,
		errorColumn:     lastError,
	}).Error
}

func ResetLogOutboxSinkByEventIDs(sink string, eventIDs []string) error {
	if len(eventIDs) == 0 {
		return nil
	}
	statusColumn, retryColumn, nextRetryColumn, errorColumn, err := logOutboxSinkUpdateColumns(sink)
	if err != nil {
		return err
	}
	return LOG_DB.Model(&LogOutbox{}).Where("event_id IN ?", eventIDs).Updates(map[string]interface{}{
		statusColumn:    LogOutboxStatusPending,
		retryColumn:     0,
		nextRetryColumn: 0,
		errorColumn:     "",
	}).Error
}

func ResetLogOutboxSinkByLogIDRange(sink string, minLogID int, maxLogID int) error {
	statusColumn, retryColumn, nextRetryColumn, errorColumn, err := logOutboxSinkUpdateColumns(sink)
	if err != nil {
		return err
	}
	tx := LOG_DB.Model(&LogOutbox{})
	if minLogID > 0 {
		tx = tx.Where("source_log_id >= ?", minLogID)
	}
	if maxLogID > 0 {
		tx = tx.Where("source_log_id <= ?", maxLogID)
	}
	return tx.Updates(map[string]interface{}{
		statusColumn:    LogOutboxStatusPending,
		retryColumn:     0,
		nextRetryColumn: 0,
		errorColumn:     "",
	}).Error
}

func ResetLogOutboxSinkByTimeRange(sink string, startTimestamp int64, endTimestamp int64) error {
	statusColumn, retryColumn, nextRetryColumn, errorColumn, err := logOutboxSinkUpdateColumns(sink)
	if err != nil {
		return err
	}
	tx := LOG_DB.Model(&LogOutbox{})
	if startTimestamp > 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	return tx.Updates(map[string]interface{}{
		statusColumn:    LogOutboxStatusPending,
		retryColumn:     0,
		nextRetryColumn: 0,
		errorColumn:     "",
	}).Error
}

func RebuildMissingLogOutboxByEventIDs(eventIDs []string) (int64, error) {
	var rebuilt int64
	for _, eventID := range eventIDs {
		eventType, logID, ok := parseStableLogEventID(eventID)
		if !ok {
			continue
		}
		var log Log
		if err := LOG_DB.First(&log, logID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return rebuilt, err
		}
		created, err := rebuildMissingLogOutboxForLog(&log, eventType)
		if err != nil {
			return rebuilt, err
		}
		if created {
			rebuilt++
		}
	}
	return rebuilt, nil
}

func RebuildMissingLogOutboxByLogIDRange(minLogID int, maxLogID int) (int64, error) {
	tx := LOG_DB.Model(&Log{}).Where("type IN ?", replayableLogTypes())
	if minLogID > 0 {
		tx = tx.Where("id >= ?", minLogID)
	}
	if maxLogID > 0 {
		tx = tx.Where("id <= ?", maxLogID)
	}
	return rebuildMissingLogOutboxByQuery(tx)
}

func RebuildMissingLogOutboxByTimeRange(startTimestamp int64, endTimestamp int64) (int64, error) {
	tx := LOG_DB.Model(&Log{}).Where("type IN ?", replayableLogTypes())
	if startTimestamp > 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	return rebuildMissingLogOutboxByQuery(tx)
}

func rebuildMissingLogOutboxByQuery(tx *gorm.DB) (int64, error) {
	var rebuilt int64
	var logs []Log
	err := tx.Order("id asc").FindInBatches(&logs, 500, func(_ *gorm.DB, _ int) error {
		for i := range logs {
			created, err := rebuildMissingLogOutboxForLog(&logs[i], "")
			if err != nil {
				return err
			}
			if created {
				rebuilt++
			}
		}
		return nil
	}).Error
	return rebuilt, err
}

func rebuildMissingLogOutboxForLog(log *Log, requestedEventType string) (bool, error) {
	if log == nil || log.Id == 0 {
		return false, nil
	}
	eventType := requestedEventType
	if eventType == "" {
		switch {
		case shouldEnqueueUsageLogOutbox(log.Type):
			eventType = LogOutboxEventTypeUsage
		case shouldEnqueueAuditLogOutbox(log.Type):
			eventType = LogOutboxEventTypeAudit
		default:
			return false, nil
		}
	}
	if eventType == LogOutboxEventTypeUsage && !shouldEnqueueUsageLogOutbox(log.Type) {
		return false, nil
	}
	if eventType == LogOutboxEventTypeAudit && !shouldEnqueueAuditLogOutbox(log.Type) {
		return false, nil
	}
	eventID := stableLogEventID(eventType, log.Id)
	var existing LogOutbox
	err := LOG_DB.Where("event_id = ?", eventID).First(&existing).Error
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	switch eventType {
	case LogOutboxEventTypeUsage:
		return true, EnqueueUsageLogOutbox(log, nil)
	case LogOutboxEventTypeAudit:
		return true, EnqueueAuditLogOutbox(log)
	default:
		return false, nil
	}
}

func replayableLogTypes() []int {
	return []int{LogTypeConsume, LogTypeError, LogTypeRefund, LogTypeManage}
}

func parseStableLogEventID(eventID string) (string, int, bool) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return "", 0, false
	}
	for _, eventType := range []string{LogOutboxEventTypeUsage, LogOutboxEventTypeAudit} {
		prefix := eventType + "-"
		if strings.HasPrefix(eventID, prefix) {
			logID, err := strconv.Atoi(strings.TrimPrefix(eventID, prefix))
			return eventType, logID, err == nil && logID > 0
		}
	}
	return "", 0, false
}

func stableLogEventID(eventType string, logID int) string {
	return fmt.Sprintf("%s-%d", eventType, logID)
}

func enqueueCanonicalLogEvent(eventID string, eventType string, log *Log, event CanonicalLogEvent) error {
	payload, err := common.Marshal(event)
	if err != nil {
		return err
	}
	metrics := logOutboxMetricsFromEvent(event)
	outbox := LogOutbox{
		EventID:               eventID,
		EventType:             eventType,
		SourceLogID:           log.Id,
		SourceRequestID:       log.RequestId,
		PayloadJSON:           LogOutboxPayloadJSON(payload),
		UsageQuota:            metrics.UsageQuota,
		UsagePromptTokens:     metrics.UsagePromptTokens,
		UsageCompletionTokens: metrics.UsageCompletionTokens,
		UsageTotalTokens:      metrics.UsageTotalTokens,
		PayloadSizeBytes:      metrics.PayloadSizeBytes,
		PayloadTruncated:      metrics.PayloadTruncated,
		KafkaStatus:           LogOutboxStatusPending,
		ClickHouseStatus:      LogOutboxStatusPending,
		KafkaNextRetryAt:      0,
		ClickHouseNextRetryAt: 0,
		CreatedAt:             log.CreatedAt,
	}
	var existing LogOutbox
	err = LOG_DB.Where("event_id = ?", eventID).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return LOG_DB.Create(&outbox).Error
}

type logOutboxMetrics struct {
	UsageQuota            int
	UsagePromptTokens     int
	UsageCompletionTokens int
	UsageTotalTokens      int
	PayloadSizeBytes      int
	PayloadTruncated      bool
}

func logOutboxMetricsFromEvent(event CanonicalLogEvent) logOutboxMetrics {
	metrics := logOutboxMetrics{}
	if event.Usage == nil {
		return metrics
	}
	metrics.UsageQuota = event.Usage.Quota
	metrics.UsagePromptTokens = event.Usage.PromptTokens
	metrics.UsageCompletionTokens = event.Usage.CompletionTokens
	metrics.UsageTotalTokens = event.Usage.PromptTokens + event.Usage.CompletionTokens
	if event.Usage.Payload != nil {
		metrics.PayloadSizeBytes = event.Usage.Payload.PayloadSizeBytes
		metrics.PayloadTruncated = event.Usage.Payload.Truncated
	}
	return metrics
}

func RefreshUsageLogOutboxPayload(eventID string, payload UsageLogPayload) error {
	if strings.TrimSpace(eventID) == "" || LOG_DB == nil {
		return nil
	}
	var outbox LogOutbox
	if err := LOG_DB.Where("event_id = ? AND event_type = ?", eventID, LogOutboxEventTypeUsage).First(&outbox).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	var event CanonicalLogEvent
	if err := common.UnmarshalJsonStr(string(outbox.PayloadJSON), &event); err != nil {
		return err
	}
	if event.Usage == nil {
		return nil
	}
	sanitized := sanitizeUsageLogPayload(payload)
	event.Usage.Payload = &sanitized
	raw, err := common.Marshal(event)
	if err != nil {
		return err
	}
	return LOG_DB.Model(&LogOutbox{}).Where("id = ?", outbox.Id).Updates(map[string]interface{}{
		"payload_json":            LogOutboxPayloadJSON(raw),
		"payload_size_bytes":      sanitized.PayloadSizeBytes,
		"payload_truncated":       sanitized.Truncated,
		"usage_quota":             event.Usage.Quota,
		"usage_prompt_tokens":     event.Usage.PromptTokens,
		"usage_completion_tokens": event.Usage.CompletionTokens,
		"usage_total_tokens":      event.Usage.PromptTokens + event.Usage.CompletionTokens,
	}).Error
}

func StripLogOutboxUsagePayload(id int64) error {
	if id <= 0 || LOG_DB == nil {
		return nil
	}
	var outbox LogOutbox
	if err := LOG_DB.First(&outbox, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if outbox.EventType != LogOutboxEventTypeUsage {
		return nil
	}
	var event CanonicalLogEvent
	if err := common.UnmarshalJsonStr(string(outbox.PayloadJSON), &event); err != nil {
		return err
	}
	if event.Usage == nil || event.Usage.Payload == nil {
		return nil
	}
	event.Usage.Payload = nil
	raw, err := common.Marshal(event)
	if err != nil {
		return err
	}
	return LOG_DB.Model(&LogOutbox{}).Where("id = ?", outbox.Id).Update("payload_json", LogOutboxPayloadJSON(raw)).Error
}

func jsonString(value interface{}) string {
	bytes, err := common.Marshal(value)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func parseLogOther(raw string) map[string]interface{} {
	if strings.TrimSpace(raw) == "" {
		return map[string]interface{}{}
	}
	other, err := common.StrToMap(raw)
	if err != nil || other == nil {
		return map[string]interface{}{}
	}
	return other
}

func requestContextFromOther(other map[string]interface{}) RequestContextSnapshot {
	var snapshot RequestContextSnapshot
	if other == nil {
		return snapshot
	}
	value, ok := other["request_context"]
	if !ok || value == nil {
		return snapshot
	}
	bytes, err := common.Marshal(value)
	if err != nil {
		return snapshot
	}
	if err := common.Unmarshal(bytes, &snapshot); err != nil {
		return RequestContextSnapshot{}
	}
	return snapshot
}

func inferAuditAction(log *Log) string {
	if log == nil {
		return ""
	}
	switch log.Type {
	case LogTypeManage:
		return "manage"
	case LogTypeSystem:
		return "system"
	case LogTypeTopup:
		return "topup"
	default:
		return "operation"
	}
}

func applyAuditFields(event *AuditLogEvent, other map[string]interface{}) {
	if event == nil || other == nil {
		return
	}
	auditRaw, ok := other["audit"]
	if !ok || auditRaw == nil {
		return
	}
	audit := map[string]interface{}{}
	bytes, err := common.Marshal(auditRaw)
	if err != nil {
		return
	}
	if err := common.Unmarshal(bytes, &audit); err != nil {
		return
	}
	event.ActorUserID = firstNonZeroInt(intFromAudit(audit, "actor_user_id"), event.ActorUserID)
	event.ActorUsername = firstNonEmptyString(stringFromAudit(audit, "actor_username"), event.ActorUsername)
	event.ActorRole = firstNonZeroInt(intFromAudit(audit, "actor_role"), event.ActorRole)
	event.ActorCompanyID = firstNonZeroInt64(int64FromAudit(audit, "actor_company_id"), event.ActorCompanyID)
	event.ActorDepartmentID = firstNonZeroInt64(int64FromAudit(audit, "actor_department_id"), event.ActorDepartmentID)
	event.TargetType = firstNonEmptyString(stringFromAudit(audit, "target_type"), event.TargetType)
	event.TargetID = firstNonEmptyString(stringFromAudit(audit, "target_id"), event.TargetID)
	event.TargetName = firstNonEmptyString(stringFromAudit(audit, "target_name"), event.TargetName)
	event.Action = firstNonEmptyString(stringFromAudit(audit, "action"), event.Action)
	event.Result = firstNonEmptyString(stringFromAudit(audit, "result"), event.Result)
	event.Summary = firstNonEmptyString(stringFromAudit(audit, "summary"), event.Summary)
	event.DiffJSON = firstNonEmptyString(stringFromAudit(audit, "diff_json"), event.DiffJSON)
	event.RequestMethod = firstNonEmptyString(stringFromAudit(audit, "request_method"), event.RequestMethod)
	event.RequestPath = firstNonEmptyString(stringFromAudit(audit, "request_path"), event.RequestPath)
	event.ClientIP = firstNonEmptyString(stringFromAudit(audit, "client_ip"), event.ClientIP)
	event.UserAgent = firstNonEmptyString(stringFromAudit(audit, "user_agent"), event.UserAgent)
	event.ApplicationID = firstNonZeroInt(intFromAudit(audit, "application_id"), event.ApplicationID)
	event.ApplicationKey = firstNonEmptyString(stringFromAudit(audit, "application_key"), event.ApplicationKey)
	event.ApplicationName = firstNonEmptyString(stringFromAudit(audit, "application_name"), event.ApplicationName)
}

func maxOutboxLagSeconds(statusColumn string, now int64) int64 {
	var row LogOutbox
	err := LOG_DB.Where(statusColumn+" IN ?", []string{LogOutboxStatusPending, LogOutboxStatusFailed, LogOutboxStatusProcessing}).
		Order("created_at asc").
		First(&row).Error
	if err != nil || row.CreatedAt <= 0 || now <= row.CreatedAt {
		return 0
	}
	return now - row.CreatedAt
}

func computePayloadStats() (int64, int64) {
	var outboxes []LogOutbox
	if err := LOG_DB.Where("event_type = ?", LogOutboxEventTypeUsage).Find(&outboxes).Error; err != nil {
		return 0, 0
	}
	var truncatedCount int64
	var sizeBytes int64
	for _, outbox := range outboxes {
		var event CanonicalLogEvent
		if err := common.UnmarshalJsonStr(string(outbox.PayloadJSON), &event); err != nil || event.Usage == nil || event.Usage.Payload == nil {
			continue
		}
		if event.Usage.Payload.Truncated {
			truncatedCount++
		}
		sizeBytes += int64(event.Usage.Payload.PayloadSizeBytes)
	}
	return truncatedCount, sizeBytes
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZeroInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func stringFromAudit(data map[string]interface{}, key string) string {
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", value)
}

func intFromAudit(data map[string]interface{}, key string) int {
	return int(int64FromAudit(data, key))
}

func int64FromAudit(data map[string]interface{}, key string) int64 {
	value, ok := data[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	default:
		return 0
	}
}

func logOutboxSinkColumns(sink string) (statusColumn string, nextRetryColumn string, err error) {
	switch sink {
	case LogOutboxSinkKafka:
		return "kafka_status", "kafka_next_retry_at", nil
	case LogOutboxSinkClickHouse:
		return "clickhouse_status", "clickhouse_next_retry_at", nil
	default:
		return "", "", fmt.Errorf("unknown log outbox sink: %s", sink)
	}
}

func logOutboxSinkUpdateColumns(sink string) (statusColumn string, retryColumn string, nextRetryColumn string, errorColumn string, err error) {
	switch sink {
	case LogOutboxSinkKafka:
		return "kafka_status", "kafka_retry_count", "kafka_next_retry_at", "kafka_last_error", nil
	case LogOutboxSinkClickHouse:
		return "clickhouse_status", "clickhouse_retry_count", "clickhouse_next_retry_at", "clickhouse_last_error", nil
	default:
		return "", "", "", "", fmt.Errorf("unknown log outbox sink: %s", sink)
	}
}

func refreshClaimedSinkStatus(event LogOutbox, sink string, nextRetryAt int64) LogOutbox {
	switch sink {
	case LogOutboxSinkKafka:
		event.KafkaStatus = LogOutboxStatusProcessing
		event.KafkaNextRetryAt = nextRetryAt
	case LogOutboxSinkClickHouse:
		event.ClickHouseStatus = LogOutboxStatusProcessing
		event.ClickHouseNextRetryAt = nextRetryAt
	}
	return event
}
