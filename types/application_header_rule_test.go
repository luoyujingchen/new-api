package types

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeApplicationHeaderValidationRulesSupportsEquals(t *testing.T) {
	rules, err := NormalizeApplicationHeaderValidationRules([]ApplicationHeaderValidationRule{
		{
			Header:   " origin ",
			Operator: "EQUALS",
			Value:    " https://app.example.com ",
			Values:   []string{"ignored"},
		},
	})

	require.NoError(t, err)
	require.Len(t, rules, 1)
	require.Equal(t, "Origin", rules[0].Header)
	require.Equal(t, ApplicationHeaderOperatorEquals, rules[0].Operator)
	require.Equal(t, "https://app.example.com", rules[0].Value)
	require.Empty(t, rules[0].Values)
}

func TestNormalizeApplicationHeaderValidationRulesSupportsOneOf(t *testing.T) {
	rules, err := NormalizeApplicationHeaderValidationRules([]ApplicationHeaderValidationRule{
		{
			Header:   " origin ",
			Operator: "ONE_OF",
			Value:    "ignored",
			Values:   []string{" https://app-a.example.com ", "", "https://app-b.example.com"},
		},
	})

	require.NoError(t, err)
	require.Len(t, rules, 1)
	require.Equal(t, "Origin", rules[0].Header)
	require.Equal(t, ApplicationHeaderOperatorOneOf, rules[0].Operator)
	require.Empty(t, rules[0].Value)
	require.Equal(t, []string{"https://app-a.example.com", "https://app-b.example.com"}, rules[0].Values)
}

func TestNormalizeApplicationHeaderValidationRulesRejectsOneOfWithoutValues(t *testing.T) {
	_, err := NormalizeApplicationHeaderValidationRules([]ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: ApplicationHeaderOperatorOneOf, Values: []string{" "}},
	})

	require.ErrorContains(t, err, "header rule values are required")
}

func TestNormalizeApplicationHeaderValidationRulesRejectsUnsupportedOperator(t *testing.T) {
	_, err := NormalizeApplicationHeaderValidationRules([]ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: "prefix", Value: "https://"},
	})

	require.ErrorContains(t, err, "unsupported header rule operator")
}

func TestMatchApplicationHeaderValidationRulesSupportsOneOf(t *testing.T) {
	headers := http.Header{}
	headers.Set("Origin", "https://app-b.example.com")

	require.True(t, MatchApplicationHeaderValidationRules(headers, []ApplicationHeaderValidationRule{
		{
			Header:   "Origin",
			Operator: ApplicationHeaderOperatorOneOf,
			Values:   []string{"https://app-a.example.com", "https://app-b.example.com"},
		},
	}))
	require.False(t, MatchApplicationHeaderValidationRules(headers, []ApplicationHeaderValidationRule{
		{
			Header:   "Origin",
			Operator: ApplicationHeaderOperatorOneOf,
			Values:   []string{"https://app-c.example.com"},
		},
	}))
}

func TestMatchApplicationHeaderValidationRulesSupportsEquals(t *testing.T) {
	headers := http.Header{}
	headers.Set("Origin", "https://app.example.com")

	require.True(t, MatchApplicationHeaderValidationRules(headers, []ApplicationHeaderValidationRule{
		{
			Header:   "Origin",
			Operator: ApplicationHeaderOperatorEquals,
			Value:    "https://app.example.com",
		},
	}))
	require.False(t, MatchApplicationHeaderValidationRules(headers, []ApplicationHeaderValidationRule{
		{
			Header:   "Origin",
			Operator: ApplicationHeaderOperatorEquals,
			Value:    "https://other.example.com",
		},
	}))
}
