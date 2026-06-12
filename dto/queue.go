package dto

import "github.com/QuantumNous/new-api/types"

type UpsertQueueConfigRequest struct {
	Enabled          *bool                        `json:"enabled" binding:"required"`
	MaxQueueSize     int                          `json:"max_queue_size"`
	QueueTimeout     int                          `json:"queue_timeout"`
	LongContextTiers []types.QueueLongContextTier `json:"long_context_tiers"`
}

type QueueConfigResponse struct {
	ModelName        string                       `json:"model_name"`
	Enabled          bool                         `json:"enabled"`
	MaxQueueSize     int                          `json:"max_queue_size"`
	QueueTimeout     int                          `json:"queue_timeout"`
	LongContextTiers []types.QueueLongContextTier `json:"long_context_tiers"`
}

type QueueModelStatusResponse struct {
	ModelName        string                             `json:"model_name"`
	Queued           int                                `json:"queued"`
	AvgWaitSec       float64                            `json:"avg_wait_sec"`
	MaxWaitSec       float64                            `json:"max_wait_sec"`
	ThroughputRPM    int                                `json:"throughput_rpm"`
	MaxQueueSize     int                                `json:"max_queue_size"`
	Enabled          bool                               `json:"enabled"`
	Buckets          map[string]int                     `json:"buckets"`
	LongContextTiers []types.QueueLongContextTierStatus `json:"long_context_tiers"`
}
