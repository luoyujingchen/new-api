package controller

import (
	"bytes"
	json "encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type queueAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setupQueueControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
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

	require.NoError(t, db.AutoMigrate(&model.QueueConfig{}))

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

func newQueueConfigContext(t *testing.T, method string, target string, modelName string, body any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	requestBody := bytes.NewReader(nil)
	if body != nil {
		payload, err := common.Marshal(body)
		require.NoError(t, err)
		requestBody = bytes.NewReader(payload)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, requestBody)
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	ctx.Params = gin.Params{{Key: "model", Value: modelName}}
	return ctx, recorder
}

func decodeQueueAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) queueAPIResponse {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var response queueAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func decodeQueueConfigResponse(t *testing.T, response queueAPIResponse) dto.QueueConfigResponse {
	t.Helper()

	require.True(t, response.Success, response.Message)
	var config dto.QueueConfigResponse
	require.NoError(t, common.Unmarshal(response.Data, &config))
	return config
}

func TestQueueConfigTimeSlotsAPIContract(t *testing.T) {
	setupQueueControllerTestDB(t)

	modelName := "deepseek-v4-flash"
	payload := map[string]any{
		"enabled":        true,
		"max_queue_size": 100,
		"queue_timeout":  55,
		"long_context_tiers": []map[string]any{
			{"threshold_tokens": 1000, "max_running": 4},
		},
		"time_slots": []map[string]any{
			{
				"start_time":     "09:00",
				"end_time":       "18:00",
				"weekdays":       []int{5, 1, 1},
				"enabled":        true,
				"max_queue_size": 3,
				"queue_timeout":  7,
				"long_context_tiers": []map[string]any{
					{"threshold_tokens": 2000, "max_running": 1},
				},
			},
			{
				"start_time":     "18:00",
				"end_time":       "20:00",
				"weekdays":       []int{1},
				"enabled":        false,
				"max_queue_size": 0,
				"queue_timeout":  0,
			},
		},
	}

	ctx, recorder := newQueueConfigContext(t, http.MethodPut, "/api/queue/config/"+modelName, modelName, payload)
	UpsertQueueConfig(ctx)
	config := decodeQueueConfigResponse(t, decodeQueueAPIResponse(t, recorder))
	require.Equal(t, modelName, config.ModelName)
	require.True(t, config.Enabled)
	require.Len(t, config.TimeSlots, 2)
	require.Equal(t, []int{1, 5}, config.TimeSlots[0].Weekdays)
	require.Equal(t, 7, config.TimeSlots[0].QueueTimeout)
	require.Len(t, config.TimeSlots[0].LongContextTiers, 1)
	require.Equal(t, 2000, config.TimeSlots[0].LongContextTiers[0].ThresholdTokens)
	require.False(t, config.TimeSlots[1].Enabled)

	ctx, recorder = newQueueConfigContext(t, http.MethodGet, "/api/queue/config/"+modelName, modelName, nil)
	GetQueueConfig(ctx)
	config = decodeQueueConfigResponse(t, decodeQueueAPIResponse(t, recorder))
	require.Len(t, config.TimeSlots, 2)
	require.Equal(t, "09:00", config.TimeSlots[0].StartTime)

	legacyPayload := map[string]any{
		"enabled":        false,
		"max_queue_size": 11,
		"queue_timeout":  22,
	}
	ctx, recorder = newQueueConfigContext(t, http.MethodPut, "/api/queue/config/"+modelName, modelName, legacyPayload)
	UpsertQueueConfig(ctx)
	config = decodeQueueConfigResponse(t, decodeQueueAPIResponse(t, recorder))
	require.False(t, config.Enabled)
	require.Equal(t, 11, config.MaxQueueSize)
	require.Equal(t, 22, config.QueueTimeout)
	require.Empty(t, config.TimeSlots)

	ctx, recorder = newQueueConfigContext(t, http.MethodDelete, "/api/queue/config/"+modelName, modelName, nil)
	DeleteQueueConfig(ctx)
	require.True(t, decodeQueueAPIResponse(t, recorder).Success)

	ctx, recorder = newQueueConfigContext(t, http.MethodGet, "/api/queue/config/"+modelName, modelName, nil)
	GetQueueConfig(ctx)
	require.False(t, decodeQueueAPIResponse(t, recorder).Success)
}

func TestQueueConfigRejectsInvalidTimeSlotPayloads(t *testing.T) {
	setupQueueControllerTestDB(t)

	cases := []struct {
		name    string
		payload map[string]any
	}{
		{
			name: "invalid time",
			payload: map[string]any{
				"enabled":        true,
				"max_queue_size": 1,
				"queue_timeout":  1,
				"time_slots": []map[string]any{
					{"start_time": "25:00", "end_time": "18:00", "enabled": true, "max_queue_size": 1, "queue_timeout": 1},
				},
			},
		},
		{
			name: "invalid weekday",
			payload: map[string]any{
				"enabled":        true,
				"max_queue_size": 1,
				"queue_timeout":  1,
				"time_slots": []map[string]any{
					{"start_time": "09:00", "end_time": "18:00", "weekdays": []int{7}, "enabled": true, "max_queue_size": 1, "queue_timeout": 1},
				},
			},
		},
		{
			name: "negative slot size",
			payload: map[string]any{
				"enabled":        true,
				"max_queue_size": 1,
				"queue_timeout":  1,
				"time_slots": []map[string]any{
					{"start_time": "09:00", "end_time": "18:00", "enabled": true, "max_queue_size": -1, "queue_timeout": 1},
				},
			},
		},
		{
			name: "invalid slot long context tier",
			payload: map[string]any{
				"enabled":        true,
				"max_queue_size": 1,
				"queue_timeout":  1,
				"time_slots": []map[string]any{
					{
						"start_time":     "09:00",
						"end_time":       "18:00",
						"enabled":        true,
						"max_queue_size": 1,
						"queue_timeout":  1,
						"long_context_tiers": []map[string]any{
							{"threshold_tokens": 0, "max_running": 1},
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, recorder := newQueueConfigContext(t, http.MethodPut, "/api/queue/config/deepseek-v4-flash", "deepseek-v4-flash", tc.payload)
			UpsertQueueConfig(ctx)
			response := decodeQueueAPIResponse(t, recorder)
			require.False(t, response.Success)
			require.NotEmpty(t, response.Message)
		})
	}
}
