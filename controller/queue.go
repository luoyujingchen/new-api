package controller

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetQueueStatus(c *gin.Context) {
	common.ApiSuccess(c, service.GetRequestQueueService().GetStatusSnapshot())
}

func getQueueModelParam(c *gin.Context) string {
	return strings.TrimSpace(strings.TrimPrefix(c.Param("model"), "/"))
}

func GetQueueModelStatus(c *gin.Context) {
	modelName := getQueueModelParam(c)
	if modelName == "" {
		common.ApiErrorMsg(c, "model is required")
		return
	}

	snapshot, ok := service.GetRequestQueueService().GetModelStatusSnapshot(modelName)
	if !ok {
		common.ApiErrorMsg(c, "model is required")
		return
	}

	common.ApiSuccess(c, dto.QueueModelStatusResponse{
		ModelName:        modelName,
		Queued:           snapshot.Queued,
		AvgWaitSec:       snapshot.AvgWaitSec,
		MaxWaitSec:       snapshot.MaxWaitSec,
		ThroughputRPM:    snapshot.ThroughputRPM,
		MaxQueueSize:     snapshot.MaxQueueSize,
		Enabled:          snapshot.Enabled,
		Buckets:          snapshot.Buckets,
		LongContextTiers: snapshot.LongContextTiers,
	})
}

func GetQueueLongContextTasks(c *gin.Context) {
	modelName := strings.TrimSpace(c.Query("model_name"))
	common.ApiSuccess(c, service.GetRequestQueueService().GetLongContextTasksSnapshot(modelName))
}

func CancelQueueLongContextTask(c *gin.Context) {
	var req dto.CancelQueueLongContextTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if !service.GetRequestQueueService().CancelLongContextTask(req.Kind, req.ID) {
		common.ApiErrorMsg(c, "long context task not found")
		return
	}
	common.ApiSuccess(c, gin.H{"cancelled": true})
}

func GetQueueConfigs(c *gin.Context) {
	configs, err := model.GetAllQueueConfigs()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	responses := make([]dto.QueueConfigResponse, 0, len(configs))
	for _, config := range configs {
		responses = append(responses, buildQueueConfigResponse(config))
	}
	common.ApiSuccess(c, responses)
}

func GetQueueConfig(c *gin.Context) {
	modelName := getQueueModelParam(c)
	config, err := model.GetQueueConfigByModelName(modelName)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "queue config not found")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildQueueConfigResponse(config))
}

func UpsertQueueConfig(c *gin.Context) {
	modelName := getQueueModelParam(c)
	if modelName == "" {
		common.ApiErrorMsg(c, "model is required")
		return
	}

	var req dto.UpsertQueueConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.MaxQueueSize < 0 {
		common.ApiErrorMsg(c, "max_queue_size must be greater than or equal to 0")
		return
	}
	if req.QueueTimeout < 0 {
		common.ApiErrorMsg(c, "queue_timeout must be greater than or equal to 0")
		return
	}
	longContextTiers, err := types.NormalizeQueueLongContextTiers(req.LongContextTiers)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	timeSlots, err := normalizeQueueTimeSlots(req.TimeSlots)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	config := &model.QueueConfig{
		ModelName:    modelName,
		Enabled:      *req.Enabled,
		MaxQueueSize: req.MaxQueueSize,
		QueueTimeout: setting.NormalizeQueueTimeoutOption(req.QueueTimeout),
	}
	if err := config.SetLongContextTiers(longContextTiers); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := config.SetTimeSlots(timeSlots); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpsertQueueConfig(config); err != nil {
		common.ApiError(c, err)
		return
	}
	service.GetRequestQueueService().NotifySchedulingRetry(modelName)
	common.ApiSuccess(c, buildQueueConfigResponse(config))
}

func DeleteQueueConfig(c *gin.Context) {
	modelName := getQueueModelParam(c)
	if modelName == "" {
		common.ApiErrorMsg(c, "model is required")
		return
	}
	if err := model.DeleteQueueConfigByModelName(modelName); err != nil {
		common.ApiError(c, err)
		return
	}
	service.GetRequestQueueService().NotifySchedulingRetry(modelName)
	common.ApiSuccess(c, nil)
}

func buildQueueConfigResponse(config *model.QueueConfig) dto.QueueConfigResponse {
	if config == nil {
		return dto.QueueConfigResponse{}
	}
	return dto.QueueConfigResponse{
		ModelName:        config.ModelName,
		Enabled:          config.Enabled,
		MaxQueueSize:     config.MaxQueueSize,
		QueueTimeout:     config.QueueTimeout,
		LongContextTiers: config.GetLongContextTiers(),
		TimeSlots:        config.GetTimeSlots(),
	}
}

func normalizeQueueTimeSlots(slots []types.QueueTimeSlotConfig) ([]types.QueueTimeSlotConfig, error) {
	normalized, err := types.NormalizeQueueTimeSlotConfigs(slots)
	if err != nil {
		return nil, err
	}
	for index := range normalized {
		normalized[index].QueueTimeout = setting.NormalizeQueueTimeoutOption(normalized[index].QueueTimeout)
	}
	return normalized, nil
}
