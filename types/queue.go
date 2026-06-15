package types

import (
	"errors"
	"sort"
)

const (
	DefaultQueueLongContextLeaseTurns              = 20
	DefaultQueueLongContextLeaseIdleTimeoutSeconds = 10
)

type QueueLongContextTier struct {
	ThresholdTokens         int `json:"threshold_tokens"`
	MaxRunning              int `json:"max_running"`
	LeaseTurns              int `json:"lease_turns"`
	LeaseIdleTimeoutSeconds int `json:"lease_idle_timeout_seconds"`
}

type QueueLongContextTierStatus struct {
	ThresholdTokens         int `json:"threshold_tokens"`
	MaxRunning              int `json:"max_running"`
	LeaseTurns              int `json:"lease_turns"`
	LeaseIdleTimeoutSeconds int `json:"lease_idle_timeout_seconds"`
	Running                 int `json:"running"`
	Queued                  int `json:"queued"`
}

func NormalizeQueueLongContextTiers(tiers []QueueLongContextTier) ([]QueueLongContextTier, error) {
	if len(tiers) == 0 {
		return nil, nil
	}

	normalized := make([]QueueLongContextTier, 0, len(tiers))
	seen := make(map[int]struct{}, len(tiers))
	for _, tier := range tiers {
		if tier.ThresholdTokens <= 0 {
			return nil, errors.New("threshold_tokens must be greater than 0")
		}
		if tier.MaxRunning <= 0 {
			return nil, errors.New("max_running must be greater than 0")
		}
		if tier.LeaseTurns < 0 {
			return nil, errors.New("lease_turns must be greater than or equal to 0")
		}
		if tier.LeaseTurns == 0 {
			tier.LeaseTurns = DefaultQueueLongContextLeaseTurns
		}
		if tier.LeaseIdleTimeoutSeconds < 0 {
			return nil, errors.New("lease_idle_timeout_seconds must be greater than or equal to 0")
		}
		if tier.LeaseIdleTimeoutSeconds == 0 {
			tier.LeaseIdleTimeoutSeconds = DefaultQueueLongContextLeaseIdleTimeoutSeconds
		}
		if _, ok := seen[tier.ThresholdTokens]; ok {
			return nil, errors.New("threshold_tokens must be unique")
		}
		seen[tier.ThresholdTokens] = struct{}{}
		normalized = append(normalized, tier)
	}

	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].ThresholdTokens < normalized[j].ThresholdTokens
	})
	for index := 1; index < len(normalized); index++ {
		if normalized[index].MaxRunning > normalized[index-1].MaxRunning {
			return nil, errors.New("higher tier max_running must not exceed lower tier max_running")
		}
	}
	return normalized, nil
}
