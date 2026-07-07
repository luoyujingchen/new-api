package service

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func setupApplicationHeaderRuleTest(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.Application{}))
	t.Cleanup(func() {
		require.NoError(t, model.DB.Exec("DELETE FROM applications").Error)
	})
}

func createApplicationWithHeaderRules(t *testing.T, name string, status int, rules []types.ApplicationHeaderValidationRule) *model.Application {
	t.Helper()
	application := &model.Application{
		AppKey: "app-" + name,
		Name:   name,
		Status: status,
	}
	require.NoError(t, application.SetHeaderValidationRules(rules))
	require.NoError(t, application.Insert())
	return application
}

func applicationHeaderRulesPtr(rules []types.ApplicationHeaderValidationRule) *[]types.ApplicationHeaderValidationRule {
	return &rules
}

func TestMatchRequestApplicationByHeadersSupportsEquals(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	application := createApplicationWithHeaderRules(t, "equals", 1, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://registered.example.com"},
	})

	headers := http.Header{}
	headers.Set("Origin", "https://registered.example.com")

	match, err := svc.MatchRequestApplicationByHeaders(headers)
	require.NoError(t, err)
	require.NotNil(t, match)
	require.True(t, match.Checked)
	require.True(t, match.Matched)
	require.NoError(t, match.Reason)
	require.NotNil(t, match.Application)
	require.Equal(t, application.Id, match.Application.Id)
}

func TestMatchRequestApplicationByHeadersSupportsOneOf(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	application := createApplicationWithHeaderRules(t, "one-of", 1, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorOneOf, Values: []string{
			"https://app-a.example.com",
			"https://app-b.example.com",
		}},
	})

	headers := http.Header{}
	headers.Set("Origin", "https://app-b.example.com")

	match, err := svc.MatchRequestApplicationByHeaders(headers)
	require.NoError(t, err)
	require.NotNil(t, match)
	require.True(t, match.Checked)
	require.True(t, match.Matched)
	require.NoError(t, match.Reason)
	require.NotNil(t, match.Application)
	require.Equal(t, application.Id, match.Application.Id)
}

func TestMatchRequestApplicationByHeadersRequiresAllRules(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	application := createApplicationWithHeaderRules(t, "multi-header", 1, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
		{Header: "X-Client-App", Operator: types.ApplicationHeaderOperatorOneOf, Values: []string{"desktop", "mobile"}},
	})

	headers := http.Header{}
	headers.Set("Origin", "https://app.example.com")
	headers.Set("X-Client-App", "desktop")

	match, err := svc.MatchRequestApplicationByHeaders(headers)
	require.NoError(t, err)
	require.NotNil(t, match)
	require.True(t, match.Checked)
	require.True(t, match.Matched)
	require.NoError(t, match.Reason)
	require.NotNil(t, match.Application)
	require.Equal(t, application.Id, match.Application.Id)

	headers.Del("X-Client-App")
	match, err = svc.MatchRequestApplicationByHeaders(headers)
	require.NoError(t, err)
	require.NotNil(t, match)
	require.True(t, match.Checked)
	require.False(t, match.Matched)
	require.ErrorIs(t, match.Reason, ErrApplicationUnrecognized)
}

func TestMatchRequestApplicationByHeadersRejectsAmbiguousApplication(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	createApplicationWithHeaderRules(t, "equals", 1, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
	})
	createApplicationWithHeaderRules(t, "one-of", 1, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorOneOf, Values: []string{
			"https://app.example.com",
			"https://other.example.com",
		}},
	})

	headers := http.Header{}
	headers.Set("Origin", "https://app.example.com")

	match, err := svc.MatchRequestApplicationByHeaders(headers)
	require.NoError(t, err)
	require.NotNil(t, match)
	require.True(t, match.Checked)
	require.True(t, match.Matched)
	require.ErrorIs(t, match.Reason, ErrApplicationHeaderAmbiguous)
}

func TestMatchRequestApplicationByHeadersRejectsUnregisteredApplication(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	createApplicationWithHeaderRules(t, "registered", 1, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://registered.example.com"},
	})

	headers := http.Header{}
	headers.Set("Origin", "https://unknown.example.com")

	match, err := svc.MatchRequestApplicationByHeaders(headers)
	require.NoError(t, err)
	require.NotNil(t, match)
	require.True(t, match.Checked)
	require.False(t, match.Matched)
	require.ErrorIs(t, match.Reason, ErrApplicationUnrecognized)
}

func TestMatchRequestApplicationByHeadersRejectsDisabledApplication(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	application := createApplicationWithHeaderRules(t, "disabled", 0, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://disabled.example.com"},
	})

	headers := http.Header{}
	headers.Set("Origin", "https://disabled.example.com")

	match, err := svc.MatchRequestApplicationByHeaders(headers)
	require.NoError(t, err)
	require.NotNil(t, match)
	require.True(t, match.Checked)
	require.True(t, match.Matched)
	require.ErrorIs(t, match.Reason, ErrApplicationDisabled)
	require.NotNil(t, match.Application)
	require.Equal(t, application.Id, match.Application.Id)
}

func TestMatchRequestApplicationByHeadersKeepsLegacyBehaviorWithoutRules(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	createApplicationWithHeaderRules(t, "no-rules", 1, nil)

	match, err := svc.MatchRequestApplicationByHeaders(http.Header{})
	require.NoError(t, err)
	require.NotNil(t, match)
	require.False(t, match.Checked)
	require.False(t, match.Matched)
	require.NoError(t, match.Reason)
}

func TestCreateApplicationRejectsOverlappingHeaderRules(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	_, err := svc.CreateApplication("existing", "", 1, 0, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
	})
	require.NoError(t, err)

	_, err = svc.CreateApplication("conflicting", "", 1, 0, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorOneOf, Values: []string{
			"https://app.example.com",
			"https://other.example.com",
		}},
	})

	require.ErrorIs(t, err, ErrApplicationHeaderConflict)
	require.Contains(t, err.Error(), "existing")
}

func TestCreateApplicationRejectsDifferentHeaderRules(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	_, err := svc.CreateApplication("origin-app", "", 1, 0, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
	})
	require.NoError(t, err)

	_, err = svc.CreateApplication("client-app", "", 1, 0, []types.ApplicationHeaderValidationRule{
		{Header: "X-Client-App", Operator: types.ApplicationHeaderOperatorEquals, Value: "desktop"},
	})

	require.ErrorIs(t, err, ErrApplicationHeaderConflict)
}

func TestCreateApplicationAllowsDisjointValuesOnSharedHeader(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	_, err := svc.CreateApplication("origin-app", "", 1, 0, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
	})
	require.NoError(t, err)

	application, err := svc.CreateApplication("other-origin-app", "", 1, 0, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorOneOf, Values: []string{
			"https://other.example.com",
			"https://another.example.com",
		}},
	})

	require.NoError(t, err)
	require.NotNil(t, application)
}

func TestUpdateApplicationRejectsOverlappingHeaderRules(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	existing, err := svc.CreateApplication("existing", "", 1, 0, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
	})
	require.NoError(t, err)
	candidate, err := svc.CreateApplication("candidate", "", 1, 0, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://candidate.example.com"},
	})
	require.NoError(t, err)

	err = svc.UpdateApplication(candidate.Id, "candidate", "", 1, 0, applicationHeaderRulesPtr([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorOneOf, Values: []string{
			"https://candidate.example.com",
			"https://app.example.com",
		}},
	}))

	require.ErrorIs(t, err, ErrApplicationHeaderConflict)
	require.Contains(t, err.Error(), existing.Name)
}

func TestUpdateApplicationPreservesHeaderRulesWhenOmitted(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	application, err := svc.CreateApplication("existing", "", 1, 0, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
	})
	require.NoError(t, err)

	err = svc.UpdateApplication(application.Id, "existing-renamed", "updated", 1, 9, nil)
	require.NoError(t, err)

	updated, err := model.GetApplicationByID(application.Id)
	require.NoError(t, err)
	rules := updated.GetHeaderValidationRules()
	require.Len(t, rules, 1)
	require.Equal(t, "Origin", rules[0].Header)
	require.Equal(t, types.ApplicationHeaderOperatorEquals, rules[0].Operator)
	require.Equal(t, "https://app.example.com", rules[0].Value)
}

func TestUpdateApplicationClearsHeaderRulesWhenExplicitlyEmpty(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	application, err := svc.CreateApplication("existing", "", 1, 0, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
	})
	require.NoError(t, err)

	err = svc.UpdateApplication(application.Id, "existing", "", 1, 0, applicationHeaderRulesPtr([]types.ApplicationHeaderValidationRule{}))
	require.NoError(t, err)

	updated, err := model.GetApplicationByID(application.Id)
	require.NoError(t, err)
	require.Empty(t, updated.GetHeaderValidationRules())
}

func TestUpdateApplicationStatusRejectsEnablingOverlappingHeaderRules(t *testing.T) {
	setupApplicationHeaderRuleTest(t)
	svc := NewApplicationService()
	enabled := createApplicationWithHeaderRules(t, "enabled", 1, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
	})
	disabled := createApplicationWithHeaderRules(t, "disabled-overlap", 0, []types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorOneOf, Values: []string{
			"https://app.example.com",
			"https://other.example.com",
		}},
	})

	err := svc.UpdateApplicationStatus(disabled.Id, 1)

	require.ErrorIs(t, err, ErrApplicationHeaderConflict)
	require.Contains(t, err.Error(), enabled.Name)
}
