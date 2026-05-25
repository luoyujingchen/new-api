package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

type QueueConfig struct {
	Id           int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ModelName    string `json:"model_name" gorm:"type:varchar(128);not null;uniqueIndex"`
	Enabled      bool   `json:"enabled" gorm:"default:true"`
	MaxQueueSize int    `json:"max_queue_size" gorm:"default:0"`
	QueueTimeout int    `json:"queue_timeout" gorm:"default:0"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func normalizeQueueModelName(modelName string) string {
	return strings.TrimSpace(modelName)
}

func GetQueueConfigByModelName(modelName string) (*QueueConfig, error) {
	modelName = normalizeQueueModelName(modelName)
	if modelName == "" {
		return nil, errors.New("model name is required")
	}
	var queueConfig QueueConfig
	err := DB.Where("model_name = ?", modelName).First(&queueConfig).Error
	if err != nil {
		return nil, err
	}
	return &queueConfig, nil
}

func GetAllQueueConfigs() ([]*QueueConfig, error) {
	var queueConfigs []*QueueConfig
	err := DB.Order("model_name ASC").Find(&queueConfigs).Error
	return queueConfigs, err
}

func UpsertQueueConfig(queueConfig *QueueConfig) error {
	if queueConfig == nil {
		return errors.New("queue config is nil")
	}
	queueConfig.ModelName = normalizeQueueModelName(queueConfig.ModelName)
	if queueConfig.ModelName == "" {
		return errors.New("model name is required")
	}

	existing, err := GetQueueConfigByModelName(queueConfig.ModelName)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return DB.Create(queueConfig).Error
	}

	existing.Enabled = queueConfig.Enabled
	existing.MaxQueueSize = queueConfig.MaxQueueSize
	existing.QueueTimeout = queueConfig.QueueTimeout
	return DB.Save(existing).Error
}

func DeleteQueueConfigByModelName(modelName string) error {
	modelName = normalizeQueueModelName(modelName)
	if modelName == "" {
		return errors.New("model name is required")
	}
	return DB.Where("model_name = ?", modelName).Delete(&QueueConfig{}).Error
}