package types

import (
	"errors"
	"fmt"
	"sort"
	"time"
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

type QueueTimeSlotConfig struct {
	StartTime        string                 `json:"start_time"`
	EndTime          string                 `json:"end_time"`
	Weekdays         []int                  `json:"weekdays,omitempty"`
	Enabled          bool                   `json:"enabled"`
	MaxQueueSize     int                    `json:"max_queue_size"`
	QueueTimeout     int                    `json:"queue_timeout"`
	LongContextTiers []QueueLongContextTier `json:"long_context_tiers"`
}

type QueueTimeSlotConfigs []QueueTimeSlotConfig

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

func NormalizeQueueTimeSlotConfigs(slots []QueueTimeSlotConfig) ([]QueueTimeSlotConfig, error) {
	if len(slots) == 0 {
		return nil, nil
	}

	normalized := make([]QueueTimeSlotConfig, 0, len(slots))
	for _, slot := range slots {
		if _, _, err := parseQueueHHMM(slot.StartTime); err != nil {
			return nil, err
		}
		if _, _, err := parseQueueHHMM(slot.EndTime); err != nil {
			return nil, err
		}
		if slot.MaxQueueSize < 0 {
			return nil, errors.New("max_queue_size must be greater than or equal to 0")
		}
		if slot.QueueTimeout < 0 {
			return nil, errors.New("queue_timeout must be greater than or equal to 0")
		}
		weekdays, err := normalizeQueueWeekdays(slot.Weekdays)
		if err != nil {
			return nil, err
		}
		longContextTiers, err := NormalizeQueueLongContextTiers(slot.LongContextTiers)
		if err != nil {
			return nil, err
		}
		slot.Weekdays = weekdays
		slot.LongContextTiers = longContextTiers
		normalized = append(normalized, slot)
	}
	return normalized, nil
}

func (slots QueueTimeSlotConfigs) MatchTimeSlot(t time.Time) int {
	weekday := int(t.Weekday())
	previousWeekday := (weekday + 6) % 7
	currentMinutes := t.Hour()*60 + t.Minute()

	for index, slot := range slots {
		startHour, startMinute, err := parseQueueHHMM(slot.StartTime)
		if err != nil {
			continue
		}
		endHour, endMinute, err := parseQueueHHMM(slot.EndTime)
		if err != nil {
			continue
		}
		startMinutes := startHour*60 + startMinute
		endMinutes := endHour*60 + endMinute

		if startMinutes <= endMinutes {
			if currentMinutes >= startMinutes && currentMinutes <= endMinutes && queueSlotMatchesWeekday(slot.Weekdays, weekday) {
				return index
			}
			continue
		}

		if currentMinutes >= startMinutes && queueSlotMatchesWeekday(slot.Weekdays, weekday) {
			return index
		}
		if currentMinutes <= endMinutes && queueSlotMatchesWeekday(slot.Weekdays, previousWeekday) {
			return index
		}
	}
	return -1
}

func normalizeQueueWeekdays(weekdays []int) ([]int, error) {
	if len(weekdays) == 0 {
		return nil, nil
	}
	seen := make(map[int]struct{}, len(weekdays))
	normalized := make([]int, 0, len(weekdays))
	for _, weekday := range weekdays {
		if weekday < 0 || weekday > 6 {
			return nil, errors.New("weekdays must be between 0 and 6")
		}
		if _, ok := seen[weekday]; ok {
			continue
		}
		seen[weekday] = struct{}{}
		normalized = append(normalized, weekday)
	}
	sort.Ints(normalized)
	return normalized, nil
}

func queueSlotMatchesWeekday(weekdays []int, weekday int) bool {
	if len(weekdays) == 0 {
		return true
	}
	for _, candidate := range weekdays {
		if candidate == weekday {
			return true
		}
	}
	return false
}

func parseQueueHHMM(value string) (hour int, minute int, err error) {
	if _, err = fmt.Sscanf(value, "%d:%d", &hour, &minute); err != nil {
		return 0, 0, err
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, errors.New("time must be in HH:MM format")
	}
	return hour, minute, nil
}
