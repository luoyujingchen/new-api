package controller

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetQueueStatus(c *gin.Context) {
	common.ApiSuccess(c, service.GetRequestQueueService().GetStatusSnapshot())
}

func GetQueueModelStatus(c *gin.Context) {
	modelName := strings.TrimSpace(c.Param("model"))
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
		ModelName:     modelName,
		Queued:        snapshot.Queued,
		AvgWaitSec:    snapshot.AvgWaitSec,
		MaxWaitSec:    snapshot.MaxWaitSec,
		ThroughputRPM: snapshot.ThroughputRPM,
		MaxQueueSize:  snapshot.MaxQueueSize,
		Enabled:       snapshot.Enabled,
		Buckets:       snapshot.Buckets,
	})
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
	modelName := strings.TrimSpace(c.Param("model"))
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
	modelName := strings.TrimSpace(c.Param("model"))
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

	config := &model.QueueConfig{
		ModelName:    modelName,
		Enabled:      *req.Enabled,
		MaxQueueSize: req.MaxQueueSize,
		QueueTimeout: setting.NormalizeQueueTimeoutOption(req.QueueTimeout),
	}
	if err := model.UpsertQueueConfig(config); err != nil {
		common.ApiError(c, err)
		return
	}
	service.GetRequestQueueService().NotifySchedulingRetry(modelName)
	common.ApiSuccess(c, buildQueueConfigResponse(config))
}

func DeleteQueueConfig(c *gin.Context) {
	modelName := strings.TrimSpace(c.Param("model"))
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
		ModelName:    config.ModelName,
		Enabled:      config.Enabled,
		MaxQueueSize: config.MaxQueueSize,
		QueueTimeout: config.QueueTimeout,
	}
}