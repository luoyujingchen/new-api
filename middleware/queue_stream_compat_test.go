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

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupQueueStreamCompatTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	oldDB := model.DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	model.DB = db
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	require.NoError(t, db.AutoMigrate(&model.QueueConfig{}, &model.Token{}, &model.User{}))

	t.Cleanup(func() {
		model.DB = oldDB
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

func TestStreamRequestQueuedByRateLimitDoesNotEmitQueueProgressEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupQueueStreamCompatTestDB(t)

	oldQueueEnabled := setting.QueueEnabled
	oldQueueDefaultTimeout := setting.QueueDefaultTimeout
	oldQueueMaxTimeout := setting.QueueMaxTimeout
	oldQueueGlobalMaxSize := setting.QueueGlobalMaxSize
	oldModelRateLimitEnabled := setting.ModelRequestRateLimitEnabled
	oldModelRateLimitCount := setting.ModelRequestRateLimitCount
	oldModelRateLimitSuccessCount := setting.ModelRequestRateLimitSuccessCount
	oldModelRateLimitDurationMinutes := setting.ModelRequestRateLimitDurationMinutes
	t.Cleanup(func() {
		setting.QueueEnabled = oldQueueEnabled
		setting.QueueDefaultTimeout = oldQueueDefaultTimeout
		setting.QueueMaxTimeout = oldQueueMaxTimeout
		setting.QueueGlobalMaxSize = oldQueueGlobalMaxSize
		setting.ModelRequestRateLimitEnabled = oldModelRateLimitEnabled
		setting.ModelRequestRateLimitCount = oldModelRateLimitCount
		setting.ModelRequestRateLimitSuccessCount = oldModelRateLimitSuccessCount
		setting.ModelRequestRateLimitDurationMinutes = oldModelRateLimitDurationMinutes
	})

	setting.QueueEnabled = true
	setting.QueueDefaultTimeout = 1
	setting.QueueMaxTimeout = 1
	setting.QueueGlobalMaxSize = 0
	setting.ModelRequestRateLimitEnabled = true
	setting.ModelRequestRateLimitCount = 0
	setting.ModelRequestRateLimitSuccessCount = 1
	setting.ModelRequestRateLimitDurationMinutes = 1

	require.NoError(t, db.Create(&model.QueueConfig{
		ModelName:    "deepseek-v4-flash",
		Enabled:      true,
		MaxQueueSize: 0,
		QueueTimeout: 1,
	}).Error)

	userID := 91042
	require.NoError(t, db.Create(&model.User{
		Id:       userID,
		Username: "queue-stream-user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:            91043,
		UserId:        userID,
		Key:           "queue-stream-token-key",
		Status:        common.TokenStatusEnabled,
		Name:          "queue-stream-token",
		Group:         "default",
		QueuePriority: 5,
		QueueTimeout:  1,
	}).Error)

	inMemoryRateLimiter.Init(time.Minute)
	require.True(t, inMemoryRateLimiter.Request(getUserRateLimitKeys(userID).SuccessKey, 1, 60))
	service.StartRequestQueueScheduler()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", "queue-user")
		c.Set("token_name", "queue-token")
		c.Set("token_queue_priority", 5)
		c.Set("token_queue_timeout", 1)
		common.SetContextKey(c, constant.ContextKeyUserId, userID)
		common.SetContextKey(c, constant.ContextKeyUserName, "queue-user")
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenId, 91043)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		c.Next()
	})
	router.Use(ModelRequestRateLimit())
	router.Use(QueueMiddleware())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"unexpected": true})
	})

	requestBody, err := common.Marshal(map[string]interface{}{
		"model": "deepseek-v4-flash",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "queued stream request",
			},
		},
		"stream": true,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(requestBody)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Queue-Timeout-Seconds", "1")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusRequestTimeout, recorder.Code, recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "event: queue")
	require.NotContains(t, recorder.Body.String(), "estimated_wait_sec")
	require.Contains(t, recorder.Body.String(), "queue_timeout")
}
