package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInMemoryRateLimiterCheck(t *testing.T) {
	var limiter InMemoryRateLimiter
	limiter.Init(0)

	require.True(t, limiter.Check("success", 0, 60))
	require.True(t, limiter.Check("success", 1, 60))
	require.True(t, limiter.Request("success", 1, 60))
	require.False(t, limiter.Check("success", 1, 60))

	// Expired windows should be considered available without mutating stored data.
	old := time.Now().Add(-2 * time.Minute).Unix()
	limiter.store["expired"] = &[]int64{old}
	require.True(t, limiter.Check("expired", 1, 60))
	require.Len(t, *limiter.store["expired"], 1)
}