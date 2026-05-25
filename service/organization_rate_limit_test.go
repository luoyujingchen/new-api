package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestTimeInRange(t *testing.T) {
	tests := []struct {
		name        string
		startTime   string
		endTime     string
		currentTime string
		expected    bool
	}{
		{"normal range inside", "09:00", "18:00", "12:00", true},
		{"normal range at start", "09:00", "18:00", "09:00", true},
		{"normal range before end", "09:00", "18:00", "17:59", true},
		{"normal range at end excluded", "09:00", "18:00", "18:00", false},
		{"normal range before start", "09:00", "18:00", "08:59", false},
		{"normal range after end", "09:00", "18:00", "18:01", false},
		{"cross-day range in evening", "23:00", "02:00", "23:30", true},
		{"cross-day range after midnight", "23:00", "02:00", "00:30", true},
		{"cross-day range before end", "23:00", "02:00", "01:59", true},
		{"cross-day range at end excluded", "23:00", "02:00", "02:00", false},
		{"cross-day range outside", "23:00", "02:00", "22:59", false},
		{"cross-day range outside midday", "23:00", "02:00", "12:00", false},
		{"full day range", "00:00", "23:59", "12:00", true},
		{"equal times match", "12:00", "12:00", "12:00", true},
		{"equal times no match", "12:00", "12:00", "11:59", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := timeInRange(tt.startTime, tt.endTime, tt.currentTime)
			if result != tt.expected {
				t.Errorf("timeInRange(%s, %s, %s) = %v, want %v",
					tt.startTime, tt.endTime, tt.currentTime, result, tt.expected)
			}
		})
	}
}

func TestWeekdayMatches(t *testing.T) {
	tests := []struct {
		name      string
		weekdays  []int
		weekday   int
		expected  bool
	}{
		{"empty matches all", []int{}, 3, true},
		{"empty matches sunday", []int{}, 0, true},
		{"matching weekday", []int{1, 2, 3, 4, 5}, 3, true},
		{"non-matching weekday", []int{1, 2, 3, 4, 5}, 6, false},
		{"single match", []int{0}, 0, true},
		{"single no match", []int{0}, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := weekdayMatches(tt.weekdays, tt.weekday)
			if result != tt.expected {
				t.Errorf("weekdayMatches(%v, %d) = %v, want %v",
					tt.weekdays, tt.weekday, result, tt.expected)
			}
		})
	}
}

func TestValidateTimeSlot(t *testing.T) {
	tests := []struct {
		name     string
		slot     dto.TimeSlot
		hasError bool
	}{
		{"valid normal", dto.TimeSlot{StartTime: "09:00", EndTime: "18:00"}, false},
		{"valid cross-day", dto.TimeSlot{StartTime: "23:00", EndTime: "02:00"}, false},
		{"valid with weekdays", dto.TimeSlot{StartTime: "09:00", EndTime: "18:00", Weekdays: []int{1, 2, 3, 4, 5}}, false},
		{"invalid start format", dto.TimeSlot{StartTime: "9:00", EndTime: "18:00"}, true},
		{"invalid end format", dto.TimeSlot{StartTime: "09:00", EndTime: "25:00"}, true},
		{"invalid weekday -1", dto.TimeSlot{StartTime: "09:00", EndTime: "18:00", Weekdays: []int{-1}}, true},
		{"invalid weekday 7", dto.TimeSlot{StartTime: "09:00", EndTime: "18:00", Weekdays: []int{7}}, true},
		{"valid weekday 0", dto.TimeSlot{StartTime: "09:00", EndTime: "18:00", Weekdays: []int{0}}, false},
		{"valid weekday 6", dto.TimeSlot{StartTime: "09:00", EndTime: "18:00", Weekdays: []int{6}}, false},
		{"invalid start hour", dto.TimeSlot{StartTime: "24:00", EndTime: "18:00"}, true},
		{"invalid start minute", dto.TimeSlot{StartTime: "09:60", EndTime: "18:00"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTimeSlot(tt.slot)
			if (err != nil) != tt.hasError {
				t.Errorf("ValidateTimeSlot(%v) error = %v, hasError %v", tt.slot, err, tt.hasError)
			}
		})
	}
}

func TestValidateCreateRateLimitRequest(t *testing.T) {
	tests := []struct {
		name     string
		req      *dto.CreateRateLimitRequest
		hasError bool
	}{
		{
			"valid",
			&dto.CreateRateLimitRequest{
				TimeSlots: []dto.TimeSlot{{StartTime: "09:00", EndTime: "18:00"}},
				Rpms:      []int{60},
			},
			false,
		},
		{
			"mismatched lengths",
			&dto.CreateRateLimitRequest{
				TimeSlots: []dto.TimeSlot{{StartTime: "09:00", EndTime: "18:00"}},
				Rpms:      []int{60, 30},
			},
			true,
		},
		{
			"negative rpm",
			&dto.CreateRateLimitRequest{
				TimeSlots: []dto.TimeSlot{{StartTime: "09:00", EndTime: "18:00"}},
				Rpms:      []int{-1},
			},
			true,
		},
		{
			"zero rpm valid",
			&dto.CreateRateLimitRequest{
				TimeSlots: []dto.TimeSlot{{StartTime: "09:00", EndTime: "18:00"}},
				Rpms:      []int{0},
			},
			false,
		},
		{
			"multiple slots valid",
			&dto.CreateRateLimitRequest{
				TimeSlots: []dto.TimeSlot{
					{StartTime: "09:00", EndTime: "18:00"},
					{StartTime: "18:00", EndTime: "23:00"},
				},
				Rpms: []int{60, 30},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCreateRateLimitRequest(tt.req)
			if (err != nil) != tt.hasError {
				t.Errorf("ValidateCreateRateLimitRequest() error = %v, hasError %v", err, tt.hasError)
			}
		})
	}
}
