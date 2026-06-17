package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLongContextQueueTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldLogConsumeEnabled := common.LogConsumeEnabled
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	model.DB = db
	model.LOG_DB = db
	common.LogConsumeEnabled = true
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	require.NoError(t, db.AutoMigrate(&model.QueueConfig{}, &model.Token{}, &model.User{}, &model.Log{}, &model.LogOutbox{}))

	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.LogConsumeEnabled = oldLogConsumeEnabled
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestDeepSeekV4FlashOver64000LongContextQueuesAndRecordsUsageLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupLongContextQueueTestDB(t)

	oldQueueEnabled := setting.QueueEnabled
	oldQueueDefaultTimeout := setting.QueueDefaultTimeout
	oldQueueMaxTimeout := setting.QueueMaxTimeout
	oldQueueGlobalMaxSize := setting.QueueGlobalMaxSize
	oldModelRateLimitEnabled := setting.ModelRequestRateLimitEnabled
	t.Cleanup(func() {
		setting.QueueEnabled = oldQueueEnabled
		setting.QueueDefaultTimeout = oldQueueDefaultTimeout
		setting.QueueMaxTimeout = oldQueueMaxTimeout
		setting.QueueGlobalMaxSize = oldQueueGlobalMaxSize
		setting.ModelRequestRateLimitEnabled = oldModelRateLimitEnabled
	})

	setting.QueueEnabled = true
	setting.QueueDefaultTimeout = 5
	setting.QueueMaxTimeout = 30
	setting.QueueGlobalMaxSize = 0
	setting.ModelRequestRateLimitEnabled = false

	queueConfig := &model.QueueConfig{
		ModelName:    "deepseek-v4-flash",
		Enabled:      true,
		MaxQueueSize: 0,
		QueueTimeout: 5,
	}
	require.NoError(t, queueConfig.SetLongContextTiers([]types.QueueLongContextTier{
		{ThresholdTokens: 64000, MaxRunning: 1},
	}))
	require.NoError(t, db.Create(queueConfig).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:            11,
		UserId:        42,
		Key:           "queue-token-key",
		Status:        common.TokenStatusEnabled,
		Name:          "queue-token",
		Group:         "default",
		QueuePriority: 5,
		QueueTimeout:  5,
	}).Error)

	service.StartRequestQueueScheduler()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", "queue-user")
		c.Set("token_name", "queue-token")
		c.Set("token_queue_priority", 5)
		c.Set("token_queue_timeout", 5)
		common.SetContextKey(c, constant.ContextKeyUserId, 42)
		common.SetContextKey(c, constant.ContextKeyUserName, "queue-user")
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenId, 11)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
		common.SetContextKey(c, constant.ContextKeyChannelId, 7)
		common.SetContextKey(c, constant.ContextKeyChannelName, "deepseek-test")
		common.SetContextKey(c, constant.ContextKeyChannelType, 43)
		c.Next()
	})
	router.Use(ModelRequestRateLimit())
	router.Use(QueueMiddleware())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		model.RecordConsumeLog(c, 42, model.RecordConsumeLogParams{
			ChannelId:        7,
			PromptTokens:     65000,
			CompletionTokens: 12,
			ModelName:        "deepseek-v4-flash",
			TokenName:        "queue-token",
			Quota:            1,
			Content:          "long context completed",
			TokenId:          11,
			UseTimeSeconds:   1,
			IsStream:         false,
			Group:            "default",
			Other: map[string]interface{}{
				"model_ratio":      1,
				"group_ratio":      1,
				"completion_ratio": 1,
			},
		})
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	requestBody, err := common.Marshal(map[string]interface{}{
		"model": "deepseek-v4-flash",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": strings.Repeat("你", 80000),
			},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(requestBody)))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var log model.Log
	require.NoError(t, db.Where("type = ? AND model_name = ?", model.LogTypeConsume, "deepseek-v4-flash").First(&log).Error)
	require.Equal(t, 65000, log.PromptTokens)
	require.Equal(t, 12, log.CompletionTokens)

	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	ctxBytes, err := common.Marshal(other["request_context"])
	require.NoError(t, err)

	var snapshot model.RequestContextSnapshot
	require.NoError(t, common.Unmarshal(ctxBytes, &snapshot))
	require.True(t, snapshot.Queue.Required)
	require.Equal(t, "deepseek-v4-flash", snapshot.Queue.ModelName)
	require.Equal(t, model.QueueResultAdmitted, snapshot.Queue.Result)
	require.GreaterOrEqual(t, snapshot.Queue.EstimatedPromptTokens, 64000)
	require.Equal(t, 64000, snapshot.Queue.MatchedLongContextTier)
}
