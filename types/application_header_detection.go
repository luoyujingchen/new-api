package types

import "strings"

const (
	ApplicationHeaderDetectionModeOff     = "off"
	ApplicationHeaderDetectionModeObserve = "observe"
	ApplicationHeaderDetectionModeEnforce = "enforce"
)

const (
	ApplicationHeaderDetectionResultSkippedOff       = "skipped_off"
	ApplicationHeaderDetectionResultSkippedNoRules   = "skipped_no_rules"
	ApplicationHeaderDetectionResultMatched          = "matched"
	ApplicationHeaderDetectionResultUnmatched        = "unmatched"
	ApplicationHeaderDetectionResultMismatch         = "mismatch"
	ApplicationHeaderDetectionResultAmbiguous        = "ambiguous"
	ApplicationHeaderDetectionResultBlockedUnmatched = "blocked_unmatched"
	ApplicationHeaderDetectionResultBlockedMismatch  = "blocked_mismatch"
	ApplicationHeaderDetectionResultBlockedAmbiguous = "blocked_ambiguous"
)

type ApplicationHeaderDetection struct {
	Mode                    string  `json:"mode"`
	Checked                 bool    `json:"checked"`
	Enforced                bool    `json:"enforced"`
	Result                  string  `json:"result"`
	Blocked                 bool    `json:"blocked"`
	Reason                  string  `json:"reason,omitempty"`
	MatchedApplicationId    int64   `json:"matched_application_id,omitempty"`
	MatchedApplicationKey   string  `json:"matched_application_key,omitempty"`
	MatchedApplicationName  string  `json:"matched_application_name,omitempty"`
	BoundApplicationId      int64   `json:"bound_application_id,omitempty"`
	BoundApplicationKey     string  `json:"bound_application_key,omitempty"`
	BoundApplicationName    string  `json:"bound_application_name,omitempty"`
	AmbiguousApplicationIds []int64 `json:"ambiguous_application_ids,omitempty"`
}

func NormalizeApplicationHeaderDetectionMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ApplicationHeaderDetectionModeObserve:
		return ApplicationHeaderDetectionModeObserve
	case ApplicationHeaderDetectionModeEnforce:
		return ApplicationHeaderDetectionModeEnforce
	default:
		return ApplicationHeaderDetectionModeOff
	}
}

func IsApplicationHeaderDetectionMode(mode string) bool {
	switch NormalizeApplicationHeaderDetectionMode(mode) {
	case ApplicationHeaderDetectionModeOff:
		return strings.EqualFold(strings.TrimSpace(mode), ApplicationHeaderDetectionModeOff)
	case ApplicationHeaderDetectionModeObserve:
		return true
	case ApplicationHeaderDetectionModeEnforce:
		return true
	default:
		return false
	}
}
