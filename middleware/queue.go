package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relayhelper "github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func QueueMiddleware() func(c *gin.Context) {
	return func(c *gin.Context) {
		if !setting.QueueEnabled || !common.GetContextKeyBool(c, constant.ContextKeyQueueRequired) {
			c.Next()
			return
		}

		modelName := common.GetContextKeyString(c, constant.ContextKeyQueueModelName)
		if modelName == "" {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, "request queue requires a resolved model name")
			return
		}

		queueService := service.GetRequestQueueService()
		effectiveConfig := queueService.GetEffectiveQueueConfig(modelName)
		if !effectiveConfig.Enabled {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, "queue is disabled for this model")
			return
		}

		tokenID := common.GetContextKeyInt(c, constant.ContextKeyTokenId)
		userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
		tokenGroup := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)

		companyID, _ := common.GetContextKeyType[int64](c, constant.ContextKeyUserCompanyId)
		tokenPriority, tokenTimeout := loadQueueTokenSettings(tokenID)
		companyPriority := loadQueueCompanyPriority(companyID)
		headerTimeout := parseQueueTimeoutHeader(c)
		isStream := detectQueueStreamRequest(c)
		estimatedPromptTokens := common.GetContextKeyInt(c, constant.ContextKeyEstimatedTokens)
		longContextTiers, _ := common.GetContextKeyType[[]types.QueueLongContextTier](c, constant.ContextKeyQueueLongContextTiers)

		var notifier service.PositionNotifier

		queuedRequest, position, _, err := queueService.Enqueue(service.QueueEnqueueOptions{
			RequestContext:        c.Request.Context(),
			ModelName:             modelName,
			TokenID:               tokenID,
			CompanyID:             companyID,
			Priority:              service.CalculateQueuePriority(tokenPriority, companyPriority),
			HeaderTimeoutSeconds:  headerTimeout,
			TokenTimeoutSeconds:   tokenTimeout,
			EstimatedPromptTokens: estimatedPromptTokens,
			LongContextTiers:      longContextTiers,
			CanProceed: func() (bool, error) {
				if !setting.ModelRequestRateLimitEnabled {
					return true, nil
				}
				config := buildModelRateLimitConfigFromValues(userID, tokenGroup, userGroup, modelName)
				allowed, _, err := tryAcquireModelRateLimit(config)
				return allowed, err
			},
			PositionNotifier: notifier,
		})
		if err != nil {
			var queueFullErr *service.QueueFullError
			if errors.As(err, &queueFullErr) {
				writeQueueError(c, http.StatusTooManyRequests, queueFullErr.Error(), "queue_full")
				return
			}
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "queue_enqueue_failed")
			return
		}

		if !isStream {
			c.Header("X-Queue-Position", strconv.Itoa(position))
		}

		select {
		case <-queuedRequest.Ready:
			queuedRequest.StopWaiting()
			defer func() {
				queueService.ReleaseLongContextSlots(queuedRequest)
				queueService.NotifySchedulingRetry(modelName)
			}()
			c.Next()
			return
		case <-queuedRequest.Context().Done():
			queuedRequest.StopWaiting()
			queueService.Remove(queuedRequest)
			if errors.Is(queuedRequest.Context().Err(), context.DeadlineExceeded) {
				positionWas := queuedRequest.Position()
				if positionWas <= 0 {
					positionWas = position
				}
				message := fmt.Sprintf("request timed out in queue after %ds (model: %s, position was %d)", int(queuedRequest.Timeout.Seconds()), modelName, positionWas)
				if isStream && c.Writer.Written() {
					writeQueueStreamError(c, message, "queue_timeout")
					c.Abort()
					return
				}
				writeQueueError(c, http.StatusRequestTimeout, message, "queue_timeout")
				return
			}
			c.Abort()
			return
		}
	}
}

func loadQueueTokenSettings(tokenID int) (priority int, timeout int) {
	priority = 5
	timeout = 0
	if tokenID == 0 {
		return priority, timeout
	}
	token, err := model.GetTokenById(tokenID)
	if err != nil {
		return priority, timeout
	}
	return setting.NormalizeQueuePriority(token.QueuePriority), setting.NormalizeQueueTimeoutOption(token.QueueTimeout)
}

func loadQueueCompanyPriority(companyID int64) int {
	if companyID == 0 {
		return 5
	}
	company, err := model.GetCompanyByID(companyID)
	if err != nil {
		return 5
	}
	return setting.NormalizeQueuePriority(company.QueuePriority)
}

func parseQueueTimeoutHeader(c *gin.Context) *int {
	raw := strings.TrimSpace(c.GetHeader("X-Queue-Timeout-Seconds"))
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return nil
	}
	return &value
}

func detectQueueStreamRequest(c *gin.Context) bool {
	relayFormat := inferQueueRelayFormat(c)
	request, err := relayhelper.GetAndValidateRequest(c, relayFormat)
	if err != nil || request == nil {
		return false
	}
	return request.IsStream(c)
}

func inferQueueRelayFormat(c *gin.Context) types.RelayFormat {
	path := c.Request.URL.Path
	switch {
	case strings.HasPrefix(path, "/v1/messages"):
		return types.RelayFormatClaude
	case strings.HasPrefix(path, "/v1/responses/compact"):
		return types.RelayFormatOpenAIResponsesCompaction
	case strings.HasPrefix(path, "/v1/responses"):
		return types.RelayFormatOpenAIResponses
	case strings.HasPrefix(path, "/v1/audio/"):
		return types.RelayFormatOpenAIAudio
	case strings.HasPrefix(path, "/v1/edits") || strings.HasPrefix(path, "/v1/images/"):
		return types.RelayFormatOpenAIImage
	case strings.HasPrefix(path, "/v1/embeddings"):
		return types.RelayFormatEmbedding
	case strings.HasPrefix(path, "/v1/rerank"):
		return types.RelayFormatRerank
	case strings.HasPrefix(path, "/v1beta/models/") || strings.HasPrefix(path, "/v1/models/") || strings.HasPrefix(path, "/v1/engines/"):
		return types.RelayFormatGemini
	default:
		return types.RelayFormatOpenAI
	}
}

func writeQueuePositionEvent(c *gin.Context, position int, estimatedWaitSec int) {
	if c == nil || c.Writer == nil || c.Request == nil {
		return
	}
	if c.Request.Context().Err() != nil {
		return
	}
	relayhelper.SetEventStreamHeaders(c)
	payload, err := common.Marshal(gin.H{
		"position":           position,
		"estimated_wait_sec": estimatedWaitSec,
	})
	if err != nil {
		return
	}
	c.Render(-1, common.CustomEvent{Data: "event: queue\n"})
	c.Render(-1, common.CustomEvent{Data: "data: " + string(payload)})
	_ = relayhelper.FlushWriter(c)
}

func writeQueueStreamError(c *gin.Context, message string, code string) {
	relayhelper.SetEventStreamHeaders(c)
	payload, err := common.Marshal(gin.H{
		"message": message,
		"type":    code,
		"code":    code,
	})
	if err != nil {
		return
	}
	c.Render(-1, common.CustomEvent{Data: "event: error\n"})
	c.Render(-1, common.CustomEvent{Data: "data: " + string(payload)})
	_ = relayhelper.FlushWriter(c)
}

func writeQueueError(c *gin.Context, statusCode int, message string, code string) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": message,
			"type":    code,
			"code":    code,
		},
	})
	c.Abort()
}
