package service

import (
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestMatchLongContextTiersCumulative(t *testing.T) {
	tiers := []types.QueueLongContextTier{
		{ThresholdTokens: 40, MaxRunning: 1},
		{ThresholdTokens: 20, MaxRunning: 4},
	}

	require.Empty(t, MatchLongContextTiers(19, tiers))

	matchedFirstTier := MatchLongContextTiers(20, tiers)
	require.Equal(t, []types.QueueLongContextTier{
		{ThresholdTokens: 20, MaxRunning: 4},
	}, matchedFirstTier)

	matchedSecondTier := MatchLongContextTiers(40, tiers)
	require.Equal(t, []types.QueueLongContextTier{
		{ThresholdTokens: 20, MaxRunning: 4},
		{ThresholdTokens: 40, MaxRunning: 1},
	}, matchedSecondTier)
}

func TestModelQueueLongContextSlotsDoNotBlockShortCandidate(t *testing.T) {
	tiers := []types.QueueLongContextTier{
		{ThresholdTokens: 20, MaxRunning: 1},
		{ThresholdTokens: 40, MaxRunning: 1},
	}
	queue := newModelQueue("gpt-test")

	firstLong := &QueuedRequest{
		ID:                    "first-long",
		Priority:              5,
		EstimatedPromptTokens: 40,
	}
	_, err := queue.Enqueue(firstLong, 0)
	require.NoError(t, err)

	candidate, _, found := queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Equal(t, firstLong, candidate)
	require.True(t, queue.Dequeue(candidate))

	secondLong := &QueuedRequest{
		ID:                    "second-long",
		Priority:              5,
		EstimatedPromptTokens: 40,
	}
	shortRequest := &QueuedRequest{
		ID:                    "short",
		Priority:              5,
		EstimatedPromptTokens: 19,
	}
	_, err = queue.Enqueue(secondLong, 0)
	require.NoError(t, err)
	_, err = queue.Enqueue(shortRequest, 0)
	require.NoError(t, err)

	candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Equal(t, shortRequest, candidate)
	require.True(t, queue.Dequeue(candidate))

	require.True(t, queue.ReleaseLongContextSlots(firstLong))
	candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Equal(t, secondLong, candidate)
	require.True(t, queue.Dequeue(candidate))
	require.True(t, queue.ReleaseLongContextSlots(candidate))
}

func TestModelQueueLongContextSlotsAreCumulativeAcrossTiers(t *testing.T) {
	tiers := []types.QueueLongContextTier{
		{ThresholdTokens: 20, MaxRunning: 2},
		{ThresholdTokens: 40, MaxRunning: 1},
	}
	queue := newModelQueue("deepseek-v4-flash")

	firstHighTier := &QueuedRequest{
		ID:                    "first-high-tier",
		Priority:              5,
		EstimatedPromptTokens: 40,
	}
	_, err := queue.Enqueue(firstHighTier, 0)
	require.NoError(t, err)
	candidate, _, found := queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Equal(t, firstHighTier, candidate)
	require.True(t, queue.Dequeue(candidate))

	secondHighTier := &QueuedRequest{
		ID:                    "second-high-tier",
		Priority:              5,
		EstimatedPromptTokens: 40,
	}
	midTier := &QueuedRequest{
		ID:                    "mid-tier",
		Priority:              5,
		EstimatedPromptTokens: 20,
	}
	shortRequest := &QueuedRequest{
		ID:                    "short",
		Priority:              5,
		EstimatedPromptTokens: 19,
	}
	_, err = queue.Enqueue(secondHighTier, 0)
	require.NoError(t, err)
	_, err = queue.Enqueue(midTier, 0)
	require.NoError(t, err)
	_, err = queue.Enqueue(shortRequest, 0)
	require.NoError(t, err)

	// The 40-token request has consumed both the 20-token and 40-token tiers.
	// Another 40-token request cannot dispatch while the 40-token tier is full,
	// but a 20-token request can still use the remaining 20-token slot.
	candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Equal(t, midTier, candidate)
	require.True(t, queue.Dequeue(candidate))

	// Now the 20-token tier is also full, so only the request below the first
	// long-task threshold can pass.
	candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Equal(t, shortRequest, candidate)
	require.True(t, queue.Dequeue(candidate))

	require.True(t, queue.ReleaseLongContextSlots(firstHighTier))
	candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Equal(t, secondHighTier, candidate)
	require.True(t, queue.Dequeue(candidate))

	require.True(t, queue.ReleaseLongContextSlots(midTier))
	require.True(t, queue.ReleaseLongContextSlots(secondHighTier))
}
