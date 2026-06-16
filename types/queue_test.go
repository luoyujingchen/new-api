package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeQueueLongContextTiers(t *testing.T) {
	normalized, err := NormalizeQueueLongContextTiers([]QueueLongContextTier{
		{ThresholdTokens: 96000, MaxRunning: 1},
		{ThresholdTokens: 64000, MaxRunning: 4},
	})
	require.NoError(t, err)
	require.Equal(t, []QueueLongContextTier{
		{ThresholdTokens: 64000, MaxRunning: 4, LeaseTurns: DefaultQueueLongContextLeaseTurns, LeaseIdleTimeoutSeconds: DefaultQueueLongContextLeaseIdleTimeoutSeconds},
		{ThresholdTokens: 96000, MaxRunning: 1, LeaseTurns: DefaultQueueLongContextLeaseTurns, LeaseIdleTimeoutSeconds: DefaultQueueLongContextLeaseIdleTimeoutSeconds},
	}, normalized)

	_, err = NormalizeQueueLongContextTiers([]QueueLongContextTier{
		{ThresholdTokens: 64000, MaxRunning: 4},
		{ThresholdTokens: 64000, MaxRunning: 1},
	})
	require.Error(t, err)

	_, err = NormalizeQueueLongContextTiers([]QueueLongContextTier{
		{ThresholdTokens: 0, MaxRunning: 1},
	})
	require.Error(t, err)

	_, err = NormalizeQueueLongContextTiers([]QueueLongContextTier{
		{ThresholdTokens: 64000, MaxRunning: 0},
	})
	require.Error(t, err)

	_, err = NormalizeQueueLongContextTiers([]QueueLongContextTier{
		{ThresholdTokens: 64000, MaxRunning: 1, LeaseTurns: -1},
	})
	require.Error(t, err)

	_, err = NormalizeQueueLongContextTiers([]QueueLongContextTier{
		{ThresholdTokens: 64000, MaxRunning: 1, LeaseIdleTimeoutSeconds: -1},
	})
	require.Error(t, err)

	_, err = NormalizeQueueLongContextTiers([]QueueLongContextTier{
		{ThresholdTokens: 96000, MaxRunning: 5},
		{ThresholdTokens: 64000, MaxRunning: 4},
	})
	require.Error(t, err)
}

func TestQueueTimeSlotConfigsMatchCrossDayUsesStartWeekday(t *testing.T) {
	slots, err := NormalizeQueueTimeSlotConfigs([]QueueTimeSlotConfig{
		{
			StartTime:    "23:00",
			EndTime:      "02:00",
			Weekdays:     []int{1},
			Enabled:      true,
			MaxQueueSize: 10,
			QueueTimeout: 30,
		},
	})
	require.NoError(t, err)

	mondayLate := time.Date(2026, time.June, 15, 23, 30, 0, 0, time.UTC)
	tuesdayEarly := time.Date(2026, time.June, 16, 1, 30, 0, 0, time.UTC)
	tuesdayLate := time.Date(2026, time.June, 16, 23, 30, 0, 0, time.UTC)

	require.Equal(t, 0, QueueTimeSlotConfigs(slots).MatchTimeSlot(mondayLate))
	require.Equal(t, 0, QueueTimeSlotConfigs(slots).MatchTimeSlot(tuesdayEarly))
	require.Equal(t, -1, QueueTimeSlotConfigs(slots).MatchTimeSlot(tuesdayLate))
}

func TestQueueTimeSlotConfigsMatchEverydayBoundaryAndFirstOverlap(t *testing.T) {
	slots, err := NormalizeQueueTimeSlotConfigs([]QueueTimeSlotConfig{
		{
			StartTime:    "09:00",
			EndTime:      "12:00",
			Enabled:      true,
			MaxQueueSize: 10,
			QueueTimeout: 30,
		},
		{
			StartTime:    "12:00",
			EndTime:      "18:00",
			Enabled:      true,
			MaxQueueSize: 20,
			QueueTimeout: 60,
		},
	})
	require.NoError(t, err)

	mondayStart := time.Date(2026, time.June, 15, 9, 0, 0, 0, time.UTC)
	mondayEndOverlap := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	mondayAfter := time.Date(2026, time.June, 15, 18, 1, 0, 0, time.UTC)

	require.Equal(t, 0, QueueTimeSlotConfigs(slots).MatchTimeSlot(mondayStart))
	require.Equal(t, 0, QueueTimeSlotConfigs(slots).MatchTimeSlot(mondayEndOverlap))
	require.Equal(t, -1, QueueTimeSlotConfigs(slots).MatchTimeSlot(mondayAfter))
}

func TestNormalizeQueueTimeSlotConfigs(t *testing.T) {
	normalized, err := NormalizeQueueTimeSlotConfigs([]QueueTimeSlotConfig{
		{
			StartTime:    "09:00",
			EndTime:      "18:00",
			Weekdays:     []int{5, 1, 1},
			Enabled:      true,
			MaxQueueSize: 10,
			QueueTimeout: 30,
			LongContextTiers: []QueueLongContextTier{
				{ThresholdTokens: 64000, MaxRunning: 2},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []int{1, 5}, normalized[0].Weekdays)
	require.Equal(t, []QueueLongContextTier{
		{
			ThresholdTokens:         64000,
			MaxRunning:              2,
			LeaseTurns:              DefaultQueueLongContextLeaseTurns,
			LeaseIdleTimeoutSeconds: DefaultQueueLongContextLeaseIdleTimeoutSeconds,
		},
	}, normalized[0].LongContextTiers)

	_, err = NormalizeQueueTimeSlotConfigs([]QueueTimeSlotConfig{
		{StartTime: "25:00", EndTime: "18:00"},
	})
	require.Error(t, err)

	_, err = NormalizeQueueTimeSlotConfigs([]QueueTimeSlotConfig{
		{StartTime: "09:00", EndTime: "18:00", Weekdays: []int{7}},
	})
	require.Error(t, err)

	_, err = NormalizeQueueTimeSlotConfigs([]QueueTimeSlotConfig{
		{StartTime: "09:00", EndTime: "18:00", MaxQueueSize: -1},
	})
	require.Error(t, err)
}
