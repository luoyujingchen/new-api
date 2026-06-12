package types

import (
	"errors"
	"sort"
)

type QueueLongContextTier struct {
	ThresholdTokens int `json:"threshold_tokens"`
	MaxRunning      int `json:"max_running"`
}

type QueueLongContextTierStatus struct {
	ThresholdTokens int `json:"threshold_tokens"`
	MaxRunning      int `json:"max_running"`
	Running         int `json:"running"`
	Queued          int `json:"queued"`
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
