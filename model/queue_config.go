package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"gorm.io/gorm"
)

type QueueConfig struct {
	Id               int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ModelName        string `json:"model_name" gorm:"type:varchar(128);not null;uniqueIndex"`
	Enabled          bool   `json:"enabled" gorm:"default:true"`
	MaxQueueSize     int    `json:"max_queue_size" gorm:"default:0"`
	QueueTimeout     int    `json:"queue_timeout" gorm:"default:0"`
	LongContextTiers string `json:"-" gorm:"type:text"`
	TimeSlots        string `json:"-" gorm:"type:text"`
	CreatedAt        int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        int64  `json:"updated_at" gorm:"autoUpdateTime"`
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
	existing.LongContextTiers = queueConfig.LongContextTiers
	existing.TimeSlots = queueConfig.TimeSlots
	return DB.Save(existing).Error
}

func DeleteQueueConfigByModelName(modelName string) error {
	modelName = normalizeQueueModelName(modelName)
	if modelName == "" {
		return errors.New("model name is required")
	}
	return DB.Where("model_name = ?", modelName).Delete(&QueueConfig{}).Error
}

func (q *QueueConfig) GetLongContextTiers() []types.QueueLongContextTier {
	if q == nil || strings.TrimSpace(q.LongContextTiers) == "" {
		return nil
	}
	var tiers []types.QueueLongContextTier
	if err := common.Unmarshal([]byte(q.LongContextTiers), &tiers); err != nil {
		return nil
	}
	normalized, err := types.NormalizeQueueLongContextTiers(tiers)
	if err != nil {
		return nil
	}
	return normalized
}

func (q *QueueConfig) SetLongContextTiers(tiers []types.QueueLongContextTier) error {
	if q == nil {
		return errors.New("queue config is nil")
	}
	normalized, err := types.NormalizeQueueLongContextTiers(tiers)
	if err != nil {
		return err
	}
	if len(normalized) == 0 {
		q.LongContextTiers = ""
		return nil
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return err
	}
	q.LongContextTiers = string(data)
	return nil
}

func (q *QueueConfig) GetTimeSlots() []types.QueueTimeSlotConfig {
	if q == nil || strings.TrimSpace(q.TimeSlots) == "" {
		return nil
	}
	var slots []types.QueueTimeSlotConfig
	if err := common.Unmarshal([]byte(q.TimeSlots), &slots); err != nil {
		return nil
	}
	normalized, err := types.NormalizeQueueTimeSlotConfigs(slots)
	if err != nil {
		return nil
	}
	return normalized
}

func (q *QueueConfig) SetTimeSlots(slots []types.QueueTimeSlotConfig) error {
	if q == nil {
		return errors.New("queue config is nil")
	}
	normalized, err := types.NormalizeQueueTimeSlotConfigs(slots)
	if err != nil {
		return err
	}
	if len(normalized) == 0 {
		q.TimeSlots = ""
		return nil
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return err
	}
	q.TimeSlots = string(data)
	return nil
}
