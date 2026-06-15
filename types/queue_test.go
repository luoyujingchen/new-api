package types

import (
	"testing"

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
