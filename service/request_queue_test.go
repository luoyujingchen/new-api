package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func testQueueTier(threshold int, maxRunning int) types.QueueLongContextTier {
	return types.QueueLongContextTier{
		ThresholdTokens:         threshold,
		MaxRunning:              maxRunning,
		LeaseTurns:              types.DefaultQueueLongContextLeaseTurns,
		LeaseIdleTimeoutSeconds: types.DefaultQueueLongContextLeaseIdleTimeoutSeconds,
	}
}

func setupRequestQueueConfigTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	oldDB := model.DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldQueueEnabled := setting.QueueEnabled
	oldQueueGlobalMaxSize := setting.QueueGlobalMaxSize

	model.DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	setting.QueueEnabled = true
	setting.QueueGlobalMaxSize = 100

	require.NoError(t, db.AutoMigrate(&model.QueueConfig{}))

	t.Cleanup(func() {
		model.DB = oldDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		setting.QueueEnabled = oldQueueEnabled
		setting.QueueGlobalMaxSize = oldQueueGlobalMaxSize

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestMatchLongContextTiersCumulative(t *testing.T) {
	tiers := []types.QueueLongContextTier{
		{ThresholdTokens: 40, MaxRunning: 1},
		{ThresholdTokens: 20, MaxRunning: 4},
	}

	require.Empty(t, MatchLongContextTiers(19, tiers))

	matchedFirstTier := MatchLongContextTiers(20, tiers)
	require.Equal(t, []types.QueueLongContextTier{
		testQueueTier(20, 4),
	}, matchedFirstTier)

	matchedSecondTier := MatchLongContextTiers(40, tiers)
	require.Equal(t, []types.QueueLongContextTier{
		testQueueTier(20, 4),
		testQueueTier(40, 1),
	}, matchedSecondTier)
}

func TestGetEffectiveQueueConfigUsesMatchingTimeSlot(t *testing.T) {
	setupRequestQueueConfigTestDB(t)

	config := &model.QueueConfig{
		ModelName:    "deepseek-v4-flash",
		Enabled:      true,
		MaxQueueSize: 99,
		QueueTimeout: 99,
	}
	require.NoError(t, config.SetLongContextTiers([]types.QueueLongContextTier{
		{ThresholdTokens: 1000, MaxRunning: 9},
	}))
	require.NoError(t, config.SetTimeSlots([]types.QueueTimeSlotConfig{
		{
			StartTime:    "09:00",
			EndTime:      "18:00",
			Weekdays:     []int{1},
			Enabled:      true,
			MaxQueueSize: 3,
			QueueTimeout: 7,
			LongContextTiers: []types.QueueLongContextTier{
				{ThresholdTokens: 2000, MaxRunning: 1},
			},
		},
	}))
	require.NoError(t, model.UpsertQueueConfig(config))

	queueService := &RequestQueueService{}
	matched := queueService.GetEffectiveQueueConfigAt(
		"deepseek-v4-flash",
		time.Date(2026, time.June, 15, 10, 0, 0, 0, time.UTC),
	)
	require.True(t, matched.Configured)
	require.True(t, matched.Enabled)
	require.Equal(t, 3, matched.MaxQueueSize)
	require.Equal(t, 7, matched.QueueTimeout)
	require.Equal(t, []types.QueueLongContextTier{
		testQueueTier(2000, 1),
	}, matched.LongContextTiers)

	unmatched := queueService.GetEffectiveQueueConfigAt(
		"deepseek-v4-flash",
		time.Date(2026, time.June, 15, 20, 0, 0, 0, time.UTC),
	)
	require.True(t, unmatched.Configured)
	require.False(t, unmatched.Enabled)
	require.Equal(t, 0, unmatched.MaxQueueSize)
	require.Equal(t, 0, unmatched.QueueTimeout)
	require.Empty(t, unmatched.LongContextTiers)
}

func TestGetEffectiveQueueConfigUsesDisabledTimeSlot(t *testing.T) {
	setupRequestQueueConfigTestDB(t)

	config := &model.QueueConfig{
		ModelName:    "deepseek-v4-flash",
		Enabled:      true,
		MaxQueueSize: 99,
		QueueTimeout: 99,
	}
	require.NoError(t, config.SetTimeSlots([]types.QueueTimeSlotConfig{
		{
			StartTime:    "00:00",
			EndTime:      "23:59",
			Weekdays:     []int{1},
			Enabled:      false,
			MaxQueueSize: 3,
			QueueTimeout: 7,
		},
	}))
	require.NoError(t, model.UpsertQueueConfig(config))

	queueService := &RequestQueueService{}
	effective := queueService.GetEffectiveQueueConfigAt(
		"deepseek-v4-flash",
		time.Date(2026, time.June, 15, 10, 0, 0, 0, time.UTC),
	)
	require.True(t, effective.Configured)
	require.False(t, effective.Enabled)
	require.Equal(t, 3, effective.MaxQueueSize)
	require.Equal(t, 7, effective.QueueTimeout)
}

func TestGetEffectiveQueueConfigTimeSlotWithoutWeekdaysMatchesEveryDay(t *testing.T) {
	setupRequestQueueConfigTestDB(t)

	config := &model.QueueConfig{
		ModelName:    "deepseek-v4-flash",
		Enabled:      false,
		MaxQueueSize: 99,
		QueueTimeout: 99,
	}
	require.NoError(t, config.SetTimeSlots([]types.QueueTimeSlotConfig{
		{
			StartTime:    "00:00",
			EndTime:      "23:59",
			Enabled:      true,
			MaxQueueSize: 8,
			QueueTimeout: 12,
		},
	}))
	require.NoError(t, model.UpsertQueueConfig(config))

	queueService := &RequestQueueService{}
	effective := queueService.GetEffectiveQueueConfigAt(
		"deepseek-v4-flash",
		time.Date(2026, time.June, 16, 10, 0, 0, 0, time.UTC),
	)
	require.True(t, effective.Configured)
	require.True(t, effective.Enabled)
	require.Equal(t, 8, effective.MaxQueueSize)
	require.Equal(t, 12, effective.QueueTimeout)
}

func TestGetEffectiveQueueConfigFallsBackToStaticWithoutTimeSlots(t *testing.T) {
	setupRequestQueueConfigTestDB(t)

	config := &model.QueueConfig{
		ModelName:    "deepseek-v4-flash",
		Enabled:      true,
		MaxQueueSize: 5,
		QueueTimeout: 11,
	}
	require.NoError(t, config.SetLongContextTiers([]types.QueueLongContextTier{
		{ThresholdTokens: 3000, MaxRunning: 2},
	}))
	require.NoError(t, model.UpsertQueueConfig(config))

	queueService := &RequestQueueService{}
	effective := queueService.GetEffectiveQueueConfigAt(
		"deepseek-v4-flash",
		time.Date(2026, time.June, 15, 20, 0, 0, 0, time.UTC),
	)
	require.True(t, effective.Configured)
	require.True(t, effective.Enabled)
	require.Equal(t, 5, effective.MaxQueueSize)
	require.Equal(t, 11, effective.QueueTimeout)
	require.Equal(t, []types.QueueLongContextTier{
		testQueueTier(3000, 2),
	}, effective.LongContextTiers)
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

func TestQueuedRequestKeepsLongContextTiersAfterWindowEnds(t *testing.T) {
	tiers := []types.QueueLongContextTier{
		{ThresholdTokens: 20, MaxRunning: 1},
	}
	queue := newModelQueue("deepseek-v4-flash")
	req := &QueuedRequest{
		ID:                    "queued-before-window-end",
		Priority:              5,
		EstimatedPromptTokens: 20,
		LongContextTiers:      MatchLongContextTiers(20, tiers),
	}
	_, err := queue.Enqueue(req, 0)
	require.NoError(t, err)

	candidate, _, found := queue.AcquireCandidateByWeight(nil, nil, nil)
	require.True(t, found)
	require.Equal(t, req, candidate)
	require.True(t, candidate.longContextSlotsAcquired)
	require.True(t, queue.Dequeue(candidate))
	require.True(t, queue.ReleaseLongContextSlots(candidate))
}

func TestLongContextLeaseRetainsSlotsUntilTurnsExhausted(t *testing.T) {
	tiers := []types.QueueLongContextTier{
		{ThresholdTokens: 20, MaxRunning: 1, LeaseTurns: 3, LeaseIdleTimeoutSeconds: 10},
	}
	queue := newModelQueue("deepseek-v4-flash")
	queueService := &RequestQueueService{
		modelQueues:       map[string]*ModelQueue{"deepseek-v4-flash": queue},
		retryEvents:       make(chan string, 10),
		longContextLeases: make(map[string]*LongContextLeaseUse),
		longContextOwners: make(map[string]map[string]struct{}),
	}

	firstLong := &QueuedRequest{
		ID:                    "first-long",
		TokenID:               11,
		CompanyID:             22,
		ModelName:             "deepseek-v4-flash",
		Priority:              5,
		EstimatedPromptTokens: 20,
	}
	_, err := queue.Enqueue(firstLong, 0)
	require.NoError(t, err)
	candidate, _, found := queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Equal(t, firstLong, candidate)
	require.True(t, queue.Dequeue(candidate))

	lease := queueService.StartLongContextLease(candidate)
	require.NotNil(t, lease)
	require.Equal(t, 2, lease.RemainingTurns)
	require.False(t, queueService.FinishLongContextLeaseUse(lease.ID))

	secondLong := &QueuedRequest{
		ID:                    "second-long",
		TokenID:               11,
		CompanyID:             22,
		ModelName:             "deepseek-v4-flash",
		Priority:              5,
		EstimatedPromptTokens: 20,
	}
	_, err = queue.Enqueue(secondLong, 0)
	require.NoError(t, err)
	candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Nil(t, candidate)

	lease, ok := queueService.TryBeginLongContextLeaseUse(LongContextLeaseUseOptions{
		ModelName:        "deepseek-v4-flash",
		TokenID:          11,
		CompanyID:        22,
		LongContextTiers: MatchLongContextTiers(20, tiers),
	})
	require.True(t, ok)
	require.Equal(t, 1, lease.RemainingTurns)
	require.False(t, queueService.FinishLongContextLeaseUse(lease.ID))

	candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Nil(t, candidate)

	lease, ok = queueService.TryBeginLongContextLeaseUse(LongContextLeaseUseOptions{
		ModelName:        "deepseek-v4-flash",
		TokenID:          11,
		CompanyID:        22,
		LongContextTiers: MatchLongContextTiers(20, tiers),
	})
	require.True(t, ok)
	require.Equal(t, 0, lease.RemainingTurns)
	require.True(t, queueService.FinishLongContextLeaseUse(lease.ID))

	candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Equal(t, secondLong, candidate)
	require.True(t, queue.Dequeue(candidate))
	require.True(t, queue.ReleaseLongContextSlots(candidate))
}

func TestLongContextLeaseIdleTimeoutReleasesSlots(t *testing.T) {
	tiers := []types.QueueLongContextTier{
		{ThresholdTokens: 20, MaxRunning: 1, LeaseTurns: 3, LeaseIdleTimeoutSeconds: 1},
	}
	queue := newModelQueue("deepseek-v4-flash")
	queueService := &RequestQueueService{
		modelQueues:       map[string]*ModelQueue{"deepseek-v4-flash": queue},
		retryEvents:       make(chan string, 10),
		longContextLeases: make(map[string]*LongContextLeaseUse),
		longContextOwners: make(map[string]map[string]struct{}),
	}

	firstLong := &QueuedRequest{
		ID:                    "first-long",
		TokenID:               11,
		CompanyID:             22,
		ModelName:             "deepseek-v4-flash",
		Priority:              5,
		EstimatedPromptTokens: 20,
	}
	_, err := queue.Enqueue(firstLong, 0)
	require.NoError(t, err)
	candidate, _, found := queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Equal(t, firstLong, candidate)
	require.True(t, queue.Dequeue(candidate))

	lease := queueService.StartLongContextLease(candidate)
	require.NotNil(t, lease)
	require.False(t, queueService.FinishLongContextLeaseUse(lease.ID))

	secondLong := &QueuedRequest{
		ID:                    "second-long",
		TokenID:               11,
		CompanyID:             22,
		ModelName:             "deepseek-v4-flash",
		Priority:              5,
		EstimatedPromptTokens: 20,
	}
	_, err = queue.Enqueue(secondLong, 0)
	require.NoError(t, err)
	candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Nil(t, candidate)

	require.Eventually(t, func() bool {
		candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
		return found && candidate == secondLong
	}, 2*time.Second, 20*time.Millisecond)
	require.True(t, queue.Dequeue(candidate))
	require.True(t, queue.ReleaseLongContextSlots(candidate))
}

func TestLongContextLeaseDoesNotReuseDifferentOwner(t *testing.T) {
	tiers := []types.QueueLongContextTier{
		{ThresholdTokens: 20, MaxRunning: 1, LeaseTurns: 3, LeaseIdleTimeoutSeconds: 10},
	}
	queue := newModelQueue("deepseek-v4-flash")
	queueService := &RequestQueueService{
		modelQueues:       map[string]*ModelQueue{"deepseek-v4-flash": queue},
		retryEvents:       make(chan string, 10),
		longContextLeases: make(map[string]*LongContextLeaseUse),
		longContextOwners: make(map[string]map[string]struct{}),
	}
	firstLong := &QueuedRequest{
		ID:                    "first-long",
		TokenID:               11,
		CompanyID:             22,
		ModelName:             "deepseek-v4-flash",
		Priority:              5,
		EstimatedPromptTokens: 20,
	}
	_, err := queue.Enqueue(firstLong, 0)
	require.NoError(t, err)
	candidate, _, found := queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.True(t, queue.Dequeue(candidate))
	lease := queueService.StartLongContextLease(candidate)
	require.NotNil(t, lease)
	require.False(t, queueService.FinishLongContextLeaseUse(lease.ID))

	_, ok := queueService.TryBeginLongContextLeaseUse(LongContextLeaseUseOptions{
		ModelName:        "deepseek-v4-flash",
		TokenID:          99,
		CompanyID:        22,
		LongContextTiers: MatchLongContextTiers(20, tiers),
	})
	require.False(t, ok)

	_, ok = queueService.TryBeginLongContextLeaseUse(LongContextLeaseUseOptions{
		ModelName:        "deepseek-v4-flash",
		TokenID:          11,
		CompanyID:        33,
		LongContextTiers: MatchLongContextTiers(20, tiers),
	})
	require.False(t, ok)

	higherTiers := []types.QueueLongContextTier{
		{ThresholdTokens: 20, MaxRunning: 1, LeaseTurns: 3, LeaseIdleTimeoutSeconds: 10},
		{ThresholdTokens: 40, MaxRunning: 1, LeaseTurns: 3, LeaseIdleTimeoutSeconds: 10},
	}
	_, ok = queueService.TryBeginLongContextLeaseUse(LongContextLeaseUseOptions{
		ModelName:        "deepseek-v4-flash",
		TokenID:          11,
		CompanyID:        22,
		LongContextTiers: MatchLongContextTiers(40, higherTiers),
	})
	require.False(t, ok)

	_, ok = queueService.TryBeginLongContextLeaseUse(LongContextLeaseUseOptions{
		ModelName:        "deepseek-v4-pro",
		TokenID:          11,
		CompanyID:        22,
		LongContextTiers: MatchLongContextTiers(20, tiers),
	})
	require.False(t, ok)
}

func TestCancelLongContextQueuedRequest(t *testing.T) {
	tiers := []types.QueueLongContextTier{
		{ThresholdTokens: 20, MaxRunning: 1, LeaseTurns: 3, LeaseIdleTimeoutSeconds: 10},
	}
	queue := newModelQueue("deepseek-v4-flash")
	queueService := &RequestQueueService{
		modelQueues:       map[string]*ModelQueue{"deepseek-v4-flash": queue},
		retryEvents:       make(chan string, 10),
		longContextLeases: make(map[string]*LongContextLeaseUse),
		longContextOwners: make(map[string]map[string]struct{}),
	}

	req := &QueuedRequest{
		ID:                    "queued-long",
		TokenID:               11,
		CompanyID:             22,
		ModelName:             "deepseek-v4-flash",
		Priority:              5,
		EstimatedPromptTokens: 20,
		EnqueuedAt:            time.Now(),
		Ready:                 make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req.ctx = ctx
	req.cancel = cancel
	_, err := queue.Enqueue(req, 0)
	require.NoError(t, err)

	queuedItems := queue.LongContextQueuedSnapshots(tiers)
	require.Len(t, queuedItems, 1)
	require.Equal(t, "queued", queuedItems[0].Kind)
	require.Equal(t, "queued-long", queuedItems[0].ID)

	require.True(t, queueService.CancelLongContextTask("queued", "queued-long"))
	require.True(t, req.IsAdminCancelled())
	require.ErrorIs(t, req.Context().Err(), context.Canceled)

	queuedItems = queue.LongContextQueuedSnapshots(tiers)
	require.Empty(t, queuedItems)
	candidate, _, found := queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.False(t, found)
	require.Nil(t, candidate)
}

func TestAdminCannotCancelRetainedLongContextLease(t *testing.T) {
	tiers := []types.QueueLongContextTier{
		{ThresholdTokens: 20, MaxRunning: 1, LeaseTurns: 3, LeaseIdleTimeoutSeconds: 10},
	}
	queue := newModelQueue("deepseek-v4-flash")
	queueService := &RequestQueueService{
		modelQueues:       map[string]*ModelQueue{"deepseek-v4-flash": queue},
		retryEvents:       make(chan string, 10),
		longContextLeases: make(map[string]*LongContextLeaseUse),
		longContextOwners: make(map[string]map[string]struct{}),
	}

	firstLong := &QueuedRequest{
		ID:                    "first-long",
		TokenID:               11,
		CompanyID:             22,
		ModelName:             "deepseek-v4-flash",
		Priority:              5,
		EstimatedPromptTokens: 20,
	}
	_, err := queue.Enqueue(firstLong, 0)
	require.NoError(t, err)
	candidate, _, found := queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.True(t, queue.Dequeue(candidate))
	lease := queueService.StartLongContextLease(candidate)
	require.NotNil(t, lease)
	require.False(t, queueService.FinishLongContextLeaseUse(lease.ID))

	secondLong := &QueuedRequest{
		ID:                    "second-long",
		TokenID:               12,
		CompanyID:             22,
		ModelName:             "deepseek-v4-flash",
		Priority:              5,
		EstimatedPromptTokens: 20,
	}
	_, err = queue.Enqueue(secondLong, 0)
	require.NoError(t, err)
	candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Nil(t, candidate)

	queuedItems := queue.LongContextQueuedSnapshots(tiers)
	require.Len(t, queuedItems, 1)
	snapshot := queueService.GetLongContextTasksSnapshot("deepseek-v4-flash")
	require.Len(t, snapshot.Items, 1)
	require.Equal(t, "leased", snapshot.Items[0].Kind)
	require.False(t, queueService.CancelLongContextTask("leased", lease.ID))

	candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Nil(t, candidate)

	require.True(t, queueService.releaseLongContextLease(lease.ID))
	candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Equal(t, secondLong, candidate)
	require.True(t, queue.Dequeue(candidate))
	require.True(t, queue.ReleaseLongContextSlots(candidate))
}

func TestRetainedLongContextLeaseQueuedRequestBypassesHeldSlot(t *testing.T) {
	tiers := []types.QueueLongContextTier{
		{ThresholdTokens: 20, MaxRunning: 1, LeaseTurns: 2, LeaseIdleTimeoutSeconds: 10},
	}
	queue := newModelQueue("deepseek-v4-flash")
	queueService := &RequestQueueService{
		modelQueues:       map[string]*ModelQueue{"deepseek-v4-flash": queue},
		retryEvents:       make(chan string, 10),
		longContextLeases: make(map[string]*LongContextLeaseUse),
		longContextOwners: make(map[string]map[string]struct{}),
	}

	firstLong := &QueuedRequest{
		ID:                    "first-long",
		TokenID:               11,
		CompanyID:             22,
		ModelName:             "deepseek-v4-flash",
		Priority:              5,
		EstimatedPromptTokens: 20,
	}
	_, err := queue.Enqueue(firstLong, 0)
	require.NoError(t, err)
	candidate, _, found := queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.True(t, queue.Dequeue(candidate))

	lease := queueService.StartLongContextLease(candidate)
	require.NotNil(t, lease)
	require.False(t, queueService.FinishLongContextLeaseUse(lease.ID))

	leaseUse, ok := queueService.TryBeginLongContextLeaseUse(LongContextLeaseUseOptions{
		ModelName:        "deepseek-v4-flash",
		TokenID:          11,
		CompanyID:        22,
		LongContextTiers: MatchLongContextTiers(20, tiers),
	})
	require.True(t, ok)
	require.Equal(t, 0, leaseUse.RemainingTurns)

	secondLong := &QueuedRequest{
		ID:                    "second-long",
		TokenID:               11,
		CompanyID:             22,
		ModelName:             "deepseek-v4-flash",
		Priority:              5,
		EstimatedPromptTokens: 20,
		retainedLeaseID:       leaseUse.ID,
	}
	_, err = queue.Enqueue(secondLong, 0)
	require.NoError(t, err)

	candidate, _, found = queue.AcquireCandidateByWeight(nil, nil, tiers)
	require.True(t, found)
	require.Equal(t, secondLong, candidate)
	require.False(t, candidate.longContextSlotsAcquired)
	require.True(t, queue.Dequeue(candidate))
	require.True(t, queueService.FinishLongContextLeaseUse(leaseUse.ID))
}
