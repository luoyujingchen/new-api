package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

type Log struct {
	Id                int    `json:"id" gorm:"index:idx_created_at_id,priority:1;index:idx_user_id_id,priority:2"`
	UserId            int    `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:2;index:idx_created_at_type"`
	Type              int    `json:"type" gorm:"index:idx_created_at_type"`
	Content           string `json:"content"`
	Username          string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName         string `json:"token_name" gorm:"index;default:''"`
	ModelName         string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota             int    `json:"quota" gorm:"default:0"`
	PromptTokens      int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens  int    `json:"completion_tokens" gorm:"default:0"`
	UseTime           int    `json:"use_time" gorm:"default:0"`
	IsStream          bool   `json:"is_stream"`
	ChannelId         int    `json:"channel" gorm:"index"`
	ChannelName       string `json:"channel_name" gorm:"->"`
	TokenId           int    `json:"token_id" gorm:"default:0;index"`
	Group             string `json:"group" gorm:"index"`
	Ip                string `json:"ip" gorm:"index;default:''"`
	RequestId         string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	UpstreamRequestId string `json:"upstream_request_id,omitempty" gorm:"type:varchar(128);index:idx_logs_upstream_request_id;default:''"`
	Other             string `json:"other"`
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
)

func appendQueueWaitInfo(c *gin.Context, other map[string]interface{}) map[string]interface{} {
	if c == nil {
		return other
	}
	queueWaitMs, ok := common.GetContextKeyType[int64](c, constant.ContextKeyQueueWaitMs)
	if !ok {
		return other
	}
	if queueWaitMs < 0 {
		queueWaitMs = 0
	}
	if other == nil {
		other = make(map[string]interface{})
	}
	other["queue_wait_ms"] = queueWaitMs
	return other
}

func formatUserLogs(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			// Remove admin-only debug fields.
			delete(otherMap, "admin_info")
			// delete(otherMap, "reject_reason")
			delete(otherMap, "stream_status")
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
		logs[i].Id = startIdx + i + 1
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	err = LOG_DB.Model(&Log{}).Where("token_id = ?", tokenId).Order("id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	formatUserLogs(logs, 0)
	return logs, err
}

func RecordLog(userId int, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
		return
	}
	enqueueLogOutboxAfterCreate(log, nil)
}

// RecordLogWithAdminInfo 记录操作日志，并将管理员相关信息存入 Other.admin_info，
func RecordLogWithAdminInfo(userId int, logType int, content string, adminInfo map[string]interface{}) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	if len(adminInfo) > 0 {
		other := map[string]interface{}{
			"admin_info": adminInfo,
		}
		log.Other = common.MapToJsonStr(other)
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record log: " + err.Error())
		return
	}
	enqueueLogOutboxAfterCreate(log, nil)
}

func RecordTopupLog(userId int, content string, callerIp string, paymentMethod string, callbackPaymentMethod string) {
	username, _ := GetUsernameById(userId, false)
	adminInfo := map[string]interface{}{
		"server_ip":               common.GetIp(),
		"node_name":               common.NodeName,
		"caller_ip":               callerIp,
		"payment_method":          paymentMethod,
		"callback_payment_method": callbackPaymentMethod,
		"version":                 common.Version,
	}
	other := map[string]interface{}{
		"admin_info": adminInfo,
	}
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeTopup,
		Content:   content,
		Ip:        callerIp,
		Other:     common.MapToJsonStr(other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record topup log: " + err.Error())
		return
	}
	enqueueLogOutboxAfterCreate(log, nil)
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, content))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	other = appendQueueWaitInfo(c, other)
	needRecordIp := ShouldRecordRequestIP(c)
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeError,
		Content:          content,
		PromptTokens:     0,
		CompletionTokens: 0,
		TokenName:        tokenName,
		ModelName:        modelName,
		Quota:            0,
		ChannelId:        channelId,
		TokenId:          tokenId,
		UseTime:          useTimeSeconds,
		IsStream:         isStream,
		Group:            group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
	}
	other = MergeRequestContextIntoOther(c, log, other)
	log.Other = common.MapToJsonStr(other)
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
		return
	}
	payload, hasPayload := GetUsageLogPayload(c)
	if hasPayload {
		enqueueLogOutboxAfterCreate(log, &payload)
	} else {
		enqueueLogOutboxAfterCreate(log, nil)
	}
	setUsageLogOutboxEventContext(c, log)
}

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(params)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	params.Other = appendQueueWaitInfo(c, params.Other)
	needRecordIp := ShouldRecordRequestIP(c)
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeConsume,
		Content:          params.Content,
		PromptTokens:     params.PromptTokens,
		CompletionTokens: params.CompletionTokens,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Quota:            params.Quota,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		Group:            params.Group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
	}
	other := MergeRequestContextIntoOther(c, log, params.Other)
	log.Other = common.MapToJsonStr(other)
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
		return
	}
	payload, hasPayload := GetUsageLogPayload(c)
	if hasPayload {
		enqueueLogOutboxAfterCreate(log, &payload)
	} else {
		enqueueLogOutboxAfterCreate(log, nil)
	}
	setUsageLogOutboxEventContext(c, log)
	if common.DataExportEnabled {
		gopool.Go(func() {
			LogQuotaData(userId, username, params.ModelName, params.Quota, common.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		})
	}
}

type RecordTaskBillingLogParams struct {
	UserId         int
	LogType        int
	Content        string
	ChannelId      int
	ModelName      string
	Quota          int
	TokenId        int
	Group          string
	Other          map[string]interface{}
	RequestContext *RequestContextSnapshot
}

func RecordTaskBillingLog(params RecordTaskBillingLogParams) {
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if token, err := GetTokenById(params.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	other := params.Other
	if other == nil {
		other = make(map[string]interface{})
	}
	if params.RequestContext != nil {
		other["request_context"] = *params.RequestContext
	}
	log := &Log{
		UserId:    params.UserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      params.LogType,
		Content:   params.Content,
		TokenName: tokenName,
		ModelName: params.ModelName,
		Quota:     params.Quota,
		ChannelId: params.ChannelId,
		TokenId:   params.TokenId,
		Group:     params.Group,
		Other:     common.MapToJsonStr(other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record task billing log: " + err.Error())
		return
	}
	enqueueLogOutboxAfterCreate(log, nil)
}

func enqueueLogOutboxAfterCreate(log *Log, payload *UsageLogPayload) {
	if log == nil {
		return
	}
	if shouldEnqueueUsageLogOutbox(log.Type) {
		if err := EnqueueUsageLogOutbox(log, payload); err != nil {
			common.SysLog("failed to enqueue usage log outbox: " + err.Error())
		}
		return
	}
	if shouldEnqueueAuditLogOutbox(log.Type) {
		if err := EnqueueAuditLogOutbox(log); err != nil {
			common.SysLog("failed to enqueue audit log outbox: " + err.Error())
		}
	}
}

func shouldEnqueueUsageLogOutbox(logType int) bool {
	return logType == LogTypeConsume || logType == LogTypeError || logType == LogTypeRefund
}

func shouldEnqueueAuditLogOutbox(logType int) bool {
	return logType == LogTypeManage
}

func setUsageLogOutboxEventContext(c *gin.Context, log *Log) {
	if c == nil || log == nil || log.Id == 0 || !shouldEnqueueUsageLogOutbox(log.Type) {
		return
	}
	common.SetContextKey(c, constant.ContextKeyUsageLogOutboxEventID, stableLogEventID(LogOutboxEventTypeUsage, log.Id))
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string, upstreamRequestId string, queueStatus string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB
	} else {
		tx = LOG_DB.Where("logs.type = ?", logType)
	}

	tx = applyLogContainsFilter(tx, "logs.model_name", modelName)
	tx = applyLogContainsFilter(tx, "logs.username", username)
	tx = applyLogContainsFilter(tx, "logs.token_name", tokenName)
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	tx = applyLogQueueStatusFilter(tx, "logs.other", queueStatus)
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if common.MemoryCacheEnabled {
			// Cache get channel
			for _, channelId := range channelIds.Items() {
				if cacheChannel, err := CacheGetChannel(channelId); err == nil {
					channels = append(channels, struct {
						Id   int    `gorm:"column:id"`
						Name string `gorm:"column:name"`
					}{
						Id:   channelId,
						Name: cacheChannel.Name,
					})
				}
			}
		} else {
			// Bulk query channels from DB
			if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
				return logs, total, err
			}
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	return logs, total, err
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string, upstreamRequestId string, queueStatus string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.Where("logs.user_id = ?", userId)
	} else {
		tx = LOG_DB.Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	tx = applyLogContainsFilter(tx, "logs.model_name", modelName)
	tx = applyLogContainsFilter(tx, "logs.token_name", tokenName)
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	tx = applyLogQueueStatusFilter(tx, "logs.other", queueStatus)
	err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error
	if err != nil {
		common.SysError("failed to count user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	formatUserLogs(logs, startIdx)
	return logs, total, err
}

type Stat struct {
	Quota       int `json:"quota"`
	Rpm         int `json:"rpm"`          // 时间段内平均每分钟请求数
	Tpm         int `json:"tpm"`          // 时间段内平均每分钟 token 数
	RealtimeRpm int `json:"realtime_rpm"` // 实时最近1分钟请求数
	RealtimeTpm int `json:"realtime_tpm"` // 实时最近1分钟 token 数
}

func logContainsPattern(input string) (string, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", false
	}

	replacer := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")
	return "%" + replacer.Replace(input) + "%", true
}

func applyLogContainsFilter(tx *gorm.DB, column string, value string) *gorm.DB {
	pattern, ok := logContainsPattern(value)
	if !ok {
		return tx
	}
	return tx.Where(column+" LIKE ? ESCAPE '!'", pattern)
}

func applyLogQueueStatusFilter(tx *gorm.DB, column string, queueStatus string) *gorm.DB {
	const queueWaitPattern = `%"queue_wait_ms"%`

	switch strings.TrimSpace(queueStatus) {
	case "queued":
		return tx.Where(column+" LIKE ?", queueWaitPattern)
	case "unqueued":
		return tx.Where("("+column+" IS NULL OR "+column+" = '' OR "+column+" NOT LIKE ?)", queueWaitPattern)
	default:
		return tx
	}
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string, queueStatus string) (stat Stat, err error) {
	// 1. 查询配额消耗
	tx := LOG_DB.Table("logs").Select("sum(quota) quota")
	tx = applyLogContainsFilter(tx, "username", username)
	tx = applyLogContainsFilter(tx, "token_name", tokenName)
	tx = applyLogContainsFilter(tx, "model_name", modelName)
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
	}
	tx = applyLogQueueStatusFilter(tx, "other", queueStatus)
	tx = tx.Where("type = ?", LogTypeConsume)

	// 2. 查询时间段内的请求数和 token 数（用于计算平均 RPM/TPM）
	periodQuery := LOG_DB.Table("logs").Select("count(*) as request_count, sum(prompt_tokens) + sum(completion_tokens) as total_tokens")
	periodQuery = applyLogContainsFilter(periodQuery, "username", username)
	periodQuery = applyLogContainsFilter(periodQuery, "token_name", tokenName)
	periodQuery = applyLogContainsFilter(periodQuery, "model_name", modelName)
	if startTimestamp != 0 {
		periodQuery = periodQuery.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		periodQuery = periodQuery.Where("created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		periodQuery = periodQuery.Where("channel_id = ?", channel)
	}
	if group != "" {
		periodQuery = periodQuery.Where(logGroupCol+" = ?", group)
	}
	periodQuery = applyLogQueueStatusFilter(periodQuery, "other", queueStatus)
	periodQuery = periodQuery.Where("type = ?", LogTypeConsume)

	// 3. 查询实时最近 1 分钟的请求数和 token 数
	realtimeQuery := LOG_DB.Table("logs").Select("count(*) as request_count, sum(prompt_tokens) + sum(completion_tokens) as total_tokens")
	realtimeQuery = applyLogContainsFilter(realtimeQuery, "username", username)
	realtimeQuery = applyLogContainsFilter(realtimeQuery, "token_name", tokenName)
	realtimeQuery = applyLogContainsFilter(realtimeQuery, "model_name", modelName)
	if channel != 0 {
		realtimeQuery = realtimeQuery.Where("channel_id = ?", channel)
	}
	if group != "" {
		realtimeQuery = realtimeQuery.Where(logGroupCol+" = ?", group)
	}
	realtimeQuery = applyLogQueueStatusFilter(realtimeQuery, "other", queueStatus)
	realtimeQuery = realtimeQuery.Where("type = ?", LogTypeConsume)
	realtimeQuery = realtimeQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 执行查询
	type periodResult struct {
		RequestCount int64 `json:"request_count"`
		TotalTokens  int64 `json:"total_tokens"`
	}

	var periodData periodResult
	var realtimeData periodResult

	if err := tx.Scan(&stat).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := periodQuery.Scan(&periodData).Error; err != nil {
		common.SysError("failed to query period stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := realtimeQuery.Scan(&realtimeData).Error; err != nil {
		common.SysError("failed to query realtime stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}

	// 计算时间段内的平均 RPM/TPM
	// 如果没有指定时间范围，默认使用 1 分钟
	var timeRangeMinutes float64
	if startTimestamp != 0 && endTimestamp != 0 && endTimestamp > startTimestamp {
		timeRangeMinutes = float64(endTimestamp-startTimestamp) / 60.0
	} else {
		timeRangeMinutes = 1.0
	}

	if timeRangeMinutes > 0 {
		stat.Rpm = int(float64(periodData.RequestCount) / timeRangeMinutes)
		stat.Tpm = int(float64(periodData.TotalTokens) / timeRangeMinutes)
	}

	// 实时 RPM/TPM 就是最近 1 分钟的绝对值
	stat.RealtimeRpm = int(realtimeData.RequestCount)
	stat.RealtimeTpm = int(realtimeData.TotalTokens)

	return stat, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := LOG_DB.Table("logs").Select("ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0
	if limit <= 0 {
		limit = 1000
	}

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&Log{})
		if nil != result.Error {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	if err := deleteOldLogOutboxes(ctx, targetTimestamp, limit); err != nil {
		return total, err
	}

	return total, nil
}

func deleteOldLogOutboxes(ctx context.Context, targetTimestamp int64, limit int) error {
	for {
		if nil != ctx.Err() {
			return ctx.Err()
		}
		var ids []int64
		if err := LOG_DB.Model(&LogOutbox{}).
			Where("created_at < ?", targetTimestamp).
			Order("id asc").
			Limit(limit).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := LOG_DB.Where("id IN ?", ids).Delete(&LogOutbox{}).Error; err != nil {
			return err
		}
		if len(ids) < limit {
			return nil
		}
	}
}
