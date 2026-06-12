package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/common/limiter"
	"github.com/QuantumNous/new-api/constant"
	relayhelper "github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type modelRateLimitConfig struct {
	durationSeconds int64
	durationMinutes int
	totalMaxCount   int
	successMaxCount int
	keys            rateLimitKeys
}

func resolveQueueModelName(c *gin.Context) string {
	if c == nil {
		return ""
	}
	modelName := common.GetContextKeyString(c, constant.ContextKeyQueueModelName)
	if modelName != "" {
		return modelName
	}
	modelRequest, _, err := getModelRequest(c)
	if err != nil || modelRequest == nil || strings.TrimSpace(modelRequest.Model) == "" {
		return ""
	}
	modelName = strings.TrimSpace(modelRequest.Model)
	common.SetContextKey(c, constant.ContextKeyQueueModelName, modelName)
	return modelName
}

func buildModelRateLimitConfig(c *gin.Context, requestModelName string) *modelRateLimitConfig {
	if c == nil {
		return nil
	}
	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	tokenGroup := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	return buildModelRateLimitConfigFromValues(userID, tokenGroup, userGroup, requestModelName)
}

func buildModelRateLimitConfigFromValues(userID int, tokenGroup string, userGroup string, requestModelName string) *modelRateLimitConfig {
	durationMinutes := setting.ModelRequestRateLimitDurationMinutes
	config := &modelRateLimitConfig{
		durationSeconds: int64(durationMinutes * 60),
		durationMinutes: durationMinutes,
		totalMaxCount:   setting.ModelRequestRateLimitCount,
		successMaxCount: setting.ModelRequestRateLimitSuccessCount,
		keys:            getUserRateLimitKeys(userID),
	}

	group := tokenGroup
	if group == "" {
		group = userGroup
	}
	if groupTotalCount, groupSuccessCount, found := setting.GetGroupRateLimit(group); found {
		config.totalMaxCount = groupTotalCount
		config.successMaxCount = groupSuccessCount
	}

	orgLimit, err := checkOrganizationRateLimit(userID, requestModelName, time.Now())
	if err == nil && orgLimit != nil && orgLimit.Rpm > 0 {
		config.totalMaxCount = orgLimit.Rpm
		config.successMaxCount = orgLimit.Rpm
		config.keys = getOrganizationRateLimitKeys(orgLimit)
	}
	return config
}

func tryAcquireModelRateLimit(config *modelRateLimitConfig) (bool, string, error) {
	if config == nil {
		return true, "", nil
	}
	if common.RedisEnabled {
		return tryAcquireRedisModelRateLimit(config)
	}
	return tryAcquireMemoryModelRateLimit(config)
}

func tryAcquireRedisModelRateLimit(config *modelRateLimitConfig) (bool, string, error) {
	ctx := context.Background()
	rdb := common.RDB

	allowed, err := checkRedisRateLimit(ctx, rdb, config.keys.SuccessKey, config.successMaxCount, config.durationSeconds)
	if err != nil {
		return false, "", err
	}
	if !allowed {
		return false, fmt.Sprintf("您已达到请求数限制：%d分钟内最多请求%d次", config.durationMinutes, config.successMaxCount), nil
	}

	if config.totalMaxCount > 0 {
		tb := limiter.New(ctx, rdb)
		allowed, err = tb.Allow(
			ctx,
			config.keys.TotalKey,
			limiter.WithCapacity(int64(config.totalMaxCount)*config.durationSeconds),
			limiter.WithRate(int64(config.totalMaxCount)),
			limiter.WithRequested(config.durationSeconds),
		)
		if err != nil {
			return false, "", err
		}
		if !allowed {
			return false, fmt.Sprintf("您已达到总请求数限制：%d分钟内最多请求%d次，包括失败次数，请检查您的请求是否正确", config.durationMinutes, config.totalMaxCount), nil
		}
	}

	return true, "", nil
}

func tryAcquireMemoryModelRateLimit(config *modelRateLimitConfig) (bool, string, error) {
	inMemoryRateLimiter.Init(time.Duration(setting.ModelRequestRateLimitDurationMinutes) * time.Minute)

	if config.successMaxCount > 0 && !inMemoryRateLimiter.Check(config.keys.SuccessKey, config.successMaxCount, config.durationSeconds) {
		return false, fmt.Sprintf("您已达到请求数限制：%d分钟内最多请求%d次", config.durationMinutes, config.successMaxCount), nil
	}
	if config.totalMaxCount > 0 && !inMemoryRateLimiter.Request(config.keys.TotalKey, config.totalMaxCount, config.durationSeconds) {
		return false, fmt.Sprintf("您已达到总请求数限制：%d分钟内最多请求%d次，包括失败次数，请检查您的请求是否正确", config.durationMinutes, config.totalMaxCount), nil
	}
	return true, "", nil
}

func recordModelRateLimitSuccess(config *modelRateLimitConfig) {
	if config == nil || config.successMaxCount <= 0 {
		return
	}
	if common.RedisEnabled {
		recordRedisRequest(context.Background(), common.RDB, config.keys.SuccessKey, config.successMaxCount)
		return
	}
	inMemoryRateLimiter.Init(time.Duration(setting.ModelRequestRateLimitDurationMinutes) * time.Minute)
	inMemoryRateLimiter.Request(config.keys.SuccessKey, config.successMaxCount, config.durationSeconds)
}

func isQueueSupportedRelayRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/pg") || strings.HasPrefix(path, "/v1/realtime") {
		return false
	}
	return true
}

func prepareLongContextQueueRequirement(c *gin.Context, modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if c == nil || modelName == "" || !setting.QueueEnabled || !isQueueSupportedRelayRequest(c) {
		return false
	}
	queueService := service.GetRequestQueueService()
	effectiveConfig := queueService.GetEffectiveQueueConfig(modelName)
	if !effectiveConfig.Enabled || len(effectiveConfig.LongContextTiers) == 0 {
		return false
	}
	promptTokens, err := estimateQueuePromptTokens(c, modelName)
	if err != nil {
		return false
	}
	common.SetContextKey(c, constant.ContextKeyEstimatedTokens, promptTokens)
	matchedTiers := service.MatchLongContextTiers(promptTokens, effectiveConfig.LongContextTiers)
	if len(matchedTiers) == 0 {
		return false
	}
	common.SetContextKey(c, constant.ContextKeyQueueRequired, true)
	common.SetContextKey(c, constant.ContextKeyQueueModelName, modelName)
	common.SetContextKey(c, constant.ContextKeyQueueLongContextTiers, matchedTiers)
	return true
}

func estimateQueuePromptTokens(c *gin.Context, modelName string) (int, error) {
	request, err := relayhelper.GetAndValidateRequest(c, inferQueueRelayFormat(c))
	if err != nil {
		return 0, err
	}
	meta := request.GetTokenCountMeta()
	if meta == nil {
		return 0, nil
	}

	tokens := 0
	if meta.TokenType == types.TokenTypeTextNumber {
		tokens += utf8.RuneCountInString(meta.CombineText)
	} else {
		tokens += service.CountTextToken(meta.CombineText, modelName)
	}

	if inferQueueRelayFormat(c) == types.RelayFormatOpenAI {
		tokens += meta.ToolsCount * 8
		tokens += meta.MessagesCount * 3
		tokens += meta.NameCount * 3
		tokens += 3
	}

	for _, file := range meta.Files {
		if file == nil {
			continue
		}
		switch file.FileType {
		case types.FileTypeImage:
			tokens += 520
		case types.FileTypeAudio:
			tokens += 256
		case types.FileTypeVideo:
			tokens += 4096 * 2
		case types.FileTypeFile:
			tokens += 4096
		default:
			tokens += 4096
		}
	}
	return tokens, nil
}
