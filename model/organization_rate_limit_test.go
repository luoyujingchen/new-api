package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeSlotsMatchCrossDayUsesStartWeekday(t *testing.T) {
	slots := TimeSlots{
		{
			StartTime: "23:00",
			EndTime:   "02:00",
			Weekdays:  []int{1},
		},
	}

	mondayLate := time.Date(2024, time.January, 1, 23, 30, 0, 0, time.UTC)
	tuesdayEarly := time.Date(2024, time.January, 2, 1, 30, 0, 0, time.UTC)
	tuesdayLate := time.Date(2024, time.January, 2, 3, 0, 0, 0, time.UTC)

	require.Equal(t, 0, slots.MatchTimeSlot(mondayLate))
	require.Equal(t, 0, slots.MatchTimeSlot(tuesdayEarly))
	require.Equal(t, -1, slots.MatchTimeSlot(tuesdayLate))
}

func TestOrganizationRateLimitValidateRejectsInvalidWeekday(t *testing.T) {
	rule := &OrganizationRateLimit{
		OrgType: "department",
		OrgId:   1,
		TimeSlots: TimeSlots{
			{
				StartTime: "09:00",
				EndTime:   "18:00",
				Weekdays:  []int{7},
			},
		},
		Rpms:   Rpms{10},
		Status: 1,
	}

	require.Error(t, rule.Validate())
}