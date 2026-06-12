package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/common/limiter"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	ModelRequestRateLimitCountMark        = "MRRL"
	ModelRequestRateLimitSuccessCountMark = "MRRLS"
)

type rateLimitKeys struct {
	TotalKey   string
	SuccessKey string
}

// 检查Redis中的请求限制
func checkRedisRateLimit(ctx context.Context, rdb *redis.Client, key string, maxCount int, duration int64) (bool, error) {
	// 如果maxCount为0，表示不限制
	if maxCount == 0 {
		return true, nil
	}

	// 获取当前计数
	length, err := rdb.LLen(ctx, key).Result()
	if err != nil {
		return false, err
	}

	// 如果未达到限制，允许请求
	if length < int64(maxCount) {
		return true, nil
	}

	// 检查时间窗口
	oldTimeStr, _ := rdb.LIndex(ctx, key, -1).Result()
	oldTime, err := time.Parse(timeFormat, oldTimeStr)
	if err != nil {
		return false, err
	}

	nowTimeStr := time.Now().Format(timeFormat)
	nowTime, err := time.Parse(timeFormat, nowTimeStr)
	if err != nil {
		return false, err
	}
	// 如果在时间窗口内已达到限制，拒绝请求
	subTime := nowTime.Sub(oldTime).Seconds()
	if int64(subTime) < duration {
		rdb.Expire(ctx, key, time.Duration(setting.ModelRequestRateLimitDurationMinutes)*time.Minute)
		return false, nil
	}

	return true, nil
}

// 记录Redis请求
func recordRedisRequest(ctx context.Context, rdb *redis.Client, key string, maxCount int) {
	// 如果maxCount为0，不记录请求
	if maxCount == 0 {
		return
	}

	now := time.Now().Format(timeFormat)
	rdb.LPush(ctx, key, now)
	rdb.LTrim(ctx, key, 0, int64(maxCount-1))
	rdb.Expire(ctx, key, time.Duration(setting.ModelRequestRateLimitDurationMinutes)*time.Minute)
}

// Redis限流处理器
func redisRateLimitHandler(duration int64, totalMaxCount, successMaxCount int, keys rateLimitKeys) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		rdb := common.RDB

		// 1. 检查成功请求数限制
		allowed, err := checkRedisRateLimit(ctx, rdb, keys.SuccessKey, successMaxCount, duration)
		if err != nil {
			fmt.Println("检查成功请求数限制失败:", err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
			return
		}
		if !allowed {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您已达到请求数限制：%d分钟内最多请求%d次", setting.ModelRequestRateLimitDurationMinutes, successMaxCount))
			return
		}

		//2.检查总请求数限制并记录总请求（当totalMaxCount为0时会自动跳过，使用令牌桶限流器
		if totalMaxCount > 0 {
			// 初始化
			tb := limiter.New(ctx, rdb)
			allowed, err = tb.Allow(
				ctx,
				keys.TotalKey,
				limiter.WithCapacity(int64(totalMaxCount)*duration),
				limiter.WithRate(int64(totalMaxCount)),
				limiter.WithRequested(duration),
			)

			if err != nil {
				fmt.Println("检查总请求数限制失败:", err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
				return
			}

			if !allowed {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您已达到总请求数限制：%d分钟内最多请求%d次，包括失败次数，请检查您的请求是否正确", setting.ModelRequestRateLimitDurationMinutes, totalMaxCount))
			}
		}

		// 4. 处理请求
		c.Next()

		// 5. 如果请求成功，记录成功请求
		if c.Writer.Status() < 400 {
			recordRedisRequest(ctx, rdb, keys.SuccessKey, successMaxCount)
		}
	}
}

// 内存限流处理器
func memoryRateLimitHandler(duration int64, totalMaxCount, successMaxCount int, keys rateLimitKeys) gin.HandlerFunc {
	inMemoryRateLimiter.Init(time.Duration(setting.ModelRequestRateLimitDurationMinutes) * time.Minute)

	return func(c *gin.Context) {
		// 1. 检查总请求数限制（当totalMaxCount为0时跳过）
		if totalMaxCount > 0 && !inMemoryRateLimiter.Request(keys.TotalKey, totalMaxCount, duration) {
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return
		}

		// 2. 检查成功请求数限制
		// 使用一个临时key来检查限制，这样可以避免实际记录
		if successMaxCount > 0 {
			if !inMemoryRateLimiter.Check(keys.SuccessKey, successMaxCount, duration) {
				c.Status(http.StatusTooManyRequests)
				c.Abort()
				return
			}
		}

		// 3. 处理请求
		c.Next()

		// 4. 如果请求成功，记录到实际的成功请求计数中
		if c.Writer.Status() < 400 && successMaxCount > 0 {
			inMemoryRateLimiter.Request(keys.SuccessKey, successMaxCount, duration)
		}
	}
}

// ModelRequestRateLimit 模型请求限流中间件
func ModelRequestRateLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		requestModelName := resolveQueueModelName(c)
		longContextQueueRequired := prepareLongContextQueueRequirement(c, requestModelName)

		if !setting.ModelRequestRateLimitEnabled {
			c.Next()
			return
		}

		config := buildModelRateLimitConfig(c, requestModelName)
		if longContextQueueRequired {
			c.Next()
			if c.Writer.Status() < http.StatusBadRequest {
				recordModelRateLimitSuccess(config)
			}
			return
		}

		allowed, rejectMessage, err := tryAcquireModelRateLimit(config)
		if err != nil {
			fmt.Println("检查请求限制失败:", err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
			return
		}
		if !allowed {
			shouldQueue := requestModelName != "" && isQueueSupportedRelayRequest(c) && setting.QueueEnabled && service.GetRequestQueueService().IsQueueEnabledForModel(requestModelName)
			if shouldQueue {
				common.SetContextKey(c, constant.ContextKeyQueueRequired, true)
				common.SetContextKey(c, constant.ContextKeyQueueModelName, requestModelName)
				c.Next()
				if c.Writer.Status() < http.StatusBadRequest {
					recordModelRateLimitSuccess(config)
				}
				return
			}
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, rejectMessage)
			return
		}

		c.Next()
		if c.Writer.Status() < http.StatusBadRequest {
			recordModelRateLimitSuccess(config)
		}
	}
}

// OrganizationLimitResult 组织限流结果
type OrganizationLimitResult struct {
	Rpm       int
	OrgType   string
	OrgId     int64
	OrgName   string
	ModelId   *int64
	ModelName string
}

// checkOrganizationRateLimit 检查用户的组织和部门限流规则
// 返回生效的限流规则，如果没有组织规则则返回 nil
func checkOrganizationRateLimit(userId int, modelName string, currentTime time.Time) (*OrganizationLimitResult, error) {
	orgRateLimitService := service.GetOrganizationRateLimitService()

	// 获取生效规则（部门优先于公司）
	effective, err := orgRateLimitService.GetEffectiveRateLimit(userId, modelName, currentTime)
	if err != nil || effective == nil {
		return nil, err
	}

	// 返回限流结果
	return &OrganizationLimitResult{
		Rpm:       effective.Rpm,
		OrgType:   effective.OrgType,
		OrgId:     effective.OrgId,
		OrgName:   effective.OrgName,
		ModelId:   effective.ModelId,
		ModelName: effective.ModelName,
	}, nil
}

func getUserRateLimitKeys(userId int) rateLimitKeys {
	userKey := strconv.Itoa(userId)
	return rateLimitKeys{
		TotalKey:   fmt.Sprintf("rateLimit:%s", userKey),
		SuccessKey: fmt.Sprintf("rateLimit:%s:%s", ModelRequestRateLimitSuccessCountMark, userKey),
	}
}

func getOrganizationRateLimitKeys(orgLimit *OrganizationLimitResult) rateLimitKeys {
	orgRateLimitService := service.GetOrganizationRateLimitService()
	return rateLimitKeys{
		TotalKey:   orgRateLimitService.GetRedisKey(orgLimit.OrgType, orgLimit.OrgId, orgLimit.ModelName, "total"),
		SuccessKey: orgRateLimitService.GetRedisKey(orgLimit.OrgType, orgLimit.OrgId, orgLimit.ModelName, "success"),
	}
}
