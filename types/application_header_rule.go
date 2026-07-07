package types

import (
	"errors"
	"net/http"
	"strings"
)

const (
	ApplicationHeaderOperatorEquals = "equals"
	ApplicationHeaderOperatorOneOf  = "one_of"
)

type ApplicationHeaderValidationRule struct {
	Header   string   `json:"header"`
	Operator string   `json:"operator"`
	Value    string   `json:"value,omitempty"`
	Values   []string `json:"values,omitempty"`
}

func NormalizeApplicationHeaderValidationRules(rules []ApplicationHeaderValidationRule) ([]ApplicationHeaderValidationRule, error) {
	normalized := make([]ApplicationHeaderValidationRule, 0, len(rules))
	for _, rule := range rules {
		header := http.CanonicalHeaderKey(strings.TrimSpace(rule.Header))
		operator := strings.ToLower(strings.TrimSpace(rule.Operator))
		value := strings.TrimSpace(rule.Value)
		values := normalizeApplicationHeaderRuleValues(rule.Values)
		if header == "" && operator == "" && value == "" && len(values) == 0 {
			continue
		}
		if header == "" {
			return nil, errors.New("header is required")
		}
		switch operator {
		case ApplicationHeaderOperatorEquals:
			if value == "" {
				return nil, errors.New("header rule value is required")
			}
			values = nil
		case ApplicationHeaderOperatorOneOf:
			if len(values) == 0 {
				return nil, errors.New("header rule values are required")
			}
			value = ""
		default:
			return nil, errors.New("unsupported header rule operator")
		}
		normalized = append(normalized, ApplicationHeaderValidationRule{
			Header:   header,
			Operator: operator,
			Value:    value,
			Values:   values,
		})
	}
	return normalized, nil
}

func normalizeApplicationHeaderRuleValues(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	return normalized
}

func MatchApplicationHeaderValidationRules(header http.Header, rules []ApplicationHeaderValidationRule) bool {
	if len(rules) == 0 {
		return false
	}
	for _, rule := range rules {
		if !matchApplicationHeaderValidationRule(header.Get(rule.Header), rule) {
			return false
		}
	}
	return true
}

func matchApplicationHeaderValidationRule(actual string, rule ApplicationHeaderValidationRule) bool {
	if actual == "" {
		return false
	}
	switch rule.Operator {
	case ApplicationHeaderOperatorEquals:
		return actual == rule.Value
	case ApplicationHeaderOperatorOneOf:
		for _, value := range rule.Values {
			if actual == value {
				return true
			}
		}
		return false
	default:
		return false
	}
}
