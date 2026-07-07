package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type applicationAPIResponse struct {
	Success bool                    `json:"success"`
	Message string                  `json:"message"`
	Data    dto.ApplicationResponse `json:"data"`
}

func setupApplicationControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	oldDB := model.DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	model.DB = db
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	require.NoError(t, db.AutoMigrate(&model.Application{}))

	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func newApplicationContext(t *testing.T, method string, target string, body any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	requestBody := bytes.NewReader(nil)
	if body != nil {
		payload, err := common.Marshal(body)
		require.NoError(t, err)
		requestBody = bytes.NewReader(payload)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, requestBody)
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	return ctx, recorder
}

func decodeApplicationAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) applicationAPIResponse {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var response applicationAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)
	return response
}

func decodeApplicationAPIError(t *testing.T, recorder *httptest.ResponseRecorder) applicationAPIResponse {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var response applicationAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
	return response
}

func applicationControllerHeaderRulesPtr(rules []types.ApplicationHeaderValidationRule) *[]types.ApplicationHeaderValidationRule {
	return &rules
}

func TestCreateApplicationPersistsHeaderValidationRules(t *testing.T) {
	setupApplicationControllerTestDB(t)

	ctx, recorder := newApplicationContext(t, http.MethodPost, "/api/application/", map[string]any{
		"name":        "Desktop App",
		"description": "desktop client",
		"sort_order":  3,
		"header_validation_rules": []types.ApplicationHeaderValidationRule{
			{
				Header:   " origin ",
				Operator: "ONE_OF",
				Values:   []string{" https://desktop.example.com ", "", "https://mobile.example.com"},
			},
		},
	})

	CreateApplication(ctx)

	response := decodeApplicationAPIResponse(t, recorder)
	require.Equal(t, "Desktop App", response.Data.Name)
	require.Equal(t, 1, response.Data.Status)
	require.Equal(t, 3, response.Data.SortOrder)
	require.Len(t, response.Data.HeaderValidationRules, 1)
	require.Equal(t, types.ApplicationHeaderOperatorOneOf, response.Data.HeaderValidationRules[0].Operator)
	require.Equal(t, []string{"https://desktop.example.com", "https://mobile.example.com"}, response.Data.HeaderValidationRules[0].Values)

	application, err := model.GetApplicationByID(response.Data.Id)
	require.NoError(t, err)
	rules := application.GetHeaderValidationRules()
	require.Len(t, rules, 1)
	require.Equal(t, "Origin", rules[0].Header)
	require.Equal(t, types.ApplicationHeaderOperatorOneOf, rules[0].Operator)
	require.Equal(t, []string{"https://desktop.example.com", "https://mobile.example.com"}, rules[0].Values)
}

func TestUpdateApplicationPersistsHeaderValidationRules(t *testing.T) {
	setupApplicationControllerTestDB(t)

	application := &model.Application{
		AppKey: "existing-app",
		Name:   "Existing App",
		Status: 1,
	}
	require.NoError(t, application.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorOneOf, Values: []string{"https://old.example.com"}},
	}))
	require.NoError(t, application.Insert())

	ctx, recorder := newApplicationContext(t, http.MethodPut, "/api/application/1", dto.UpdateApplicationRequest{
		Name:        "Updated App",
		Description: "updated",
		Status:      0,
		SortOrder:   7,
		HeaderValidationRules: applicationControllerHeaderRulesPtr([]types.ApplicationHeaderValidationRule{
			{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://updated.example.com"},
		}),
	})
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(application.Id, 10)}}

	UpdateApplication(ctx)

	response := decodeApplicationAPIResponse(t, recorder)
	require.Equal(t, application.Id, response.Data.Id)
	require.Equal(t, "Updated App", response.Data.Name)
	require.Equal(t, 0, response.Data.Status)
	require.Len(t, response.Data.HeaderValidationRules, 1)
	require.Equal(t, types.ApplicationHeaderOperatorEquals, response.Data.HeaderValidationRules[0].Operator)
	require.Equal(t, "https://updated.example.com", response.Data.HeaderValidationRules[0].Value)

	updated, err := model.GetApplicationByID(application.Id)
	require.NoError(t, err)
	require.Equal(t, 0, updated.Status)
	rules := updated.GetHeaderValidationRules()
	require.Len(t, rules, 1)
	require.Equal(t, types.ApplicationHeaderOperatorEquals, rules[0].Operator)
	require.Equal(t, "https://updated.example.com", rules[0].Value)
	require.Empty(t, rules[0].Values)
}

func TestUpdateApplicationPreservesHeaderValidationRulesWhenOmitted(t *testing.T) {
	setupApplicationControllerTestDB(t)

	application := &model.Application{
		AppKey: "existing-app",
		Name:   "Existing App",
		Status: 1,
	}
	require.NoError(t, application.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://preserve.example.com"},
	}))
	require.NoError(t, application.Insert())

	ctx, recorder := newApplicationContext(t, http.MethodPut, "/api/application/"+strconv.FormatInt(application.Id, 10), map[string]any{
		"name":        "Updated App",
		"description": "updated without rules",
		"status":      1,
		"sort_order":  4,
	})
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(application.Id, 10)}}

	UpdateApplication(ctx)

	response := decodeApplicationAPIResponse(t, recorder)
	require.Len(t, response.Data.HeaderValidationRules, 1)
	require.Equal(t, "https://preserve.example.com", response.Data.HeaderValidationRules[0].Value)

	updated, err := model.GetApplicationByID(application.Id)
	require.NoError(t, err)
	rules := updated.GetHeaderValidationRules()
	require.Len(t, rules, 1)
	require.Equal(t, "https://preserve.example.com", rules[0].Value)
}

func TestUpdateApplicationClearsHeaderValidationRulesWhenExplicitlyEmpty(t *testing.T) {
	setupApplicationControllerTestDB(t)

	application := &model.Application{
		AppKey: "existing-app",
		Name:   "Existing App",
		Status: 1,
	}
	require.NoError(t, application.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://clear.example.com"},
	}))
	require.NoError(t, application.Insert())

	ctx, recorder := newApplicationContext(t, http.MethodPut, "/api/application/"+strconv.FormatInt(application.Id, 10), map[string]any{
		"name":                    "Updated App",
		"status":                  1,
		"sort_order":              0,
		"header_validation_rules": []types.ApplicationHeaderValidationRule{},
	})
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(application.Id, 10)}}

	UpdateApplication(ctx)

	response := decodeApplicationAPIResponse(t, recorder)
	require.Empty(t, response.Data.HeaderValidationRules)

	updated, err := model.GetApplicationByID(application.Id)
	require.NoError(t, err)
	require.Empty(t, updated.GetHeaderValidationRules())
}

func TestCreateApplicationRejectsConflictingHeaderValidationRules(t *testing.T) {
	setupApplicationControllerTestDB(t)

	application := &model.Application{
		AppKey: "existing-app",
		Name:   "Existing App",
		Status: 1,
	}
	require.NoError(t, application.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
	}))
	require.NoError(t, application.Insert())

	ctx, recorder := newApplicationContext(t, http.MethodPost, "/api/application/", map[string]any{
		"name":   "Conflicting App",
		"status": 1,
		"header_validation_rules": []types.ApplicationHeaderValidationRule{
			{Header: "Origin", Operator: types.ApplicationHeaderOperatorOneOf, Values: []string{
				"https://app.example.com",
				"https://other.example.com",
			}},
		},
	})

	CreateApplication(ctx)

	response := decodeApplicationAPIError(t, recorder)
	require.Contains(t, response.Message, "application header rules conflict")
	require.Contains(t, response.Message, application.Name)
}

func TestCreateApplicationRejectsDifferentHeaderValidationRules(t *testing.T) {
	setupApplicationControllerTestDB(t)

	application := &model.Application{
		AppKey: "existing-app",
		Name:   "Existing App",
		Status: 1,
	}
	require.NoError(t, application.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
	}))
	require.NoError(t, application.Insert())

	ctx, recorder := newApplicationContext(t, http.MethodPost, "/api/application/", map[string]any{
		"name":   "Different Header App",
		"status": 1,
		"header_validation_rules": []types.ApplicationHeaderValidationRule{
			{Header: "X-Client-App", Operator: types.ApplicationHeaderOperatorEquals, Value: "desktop"},
		},
	})

	CreateApplication(ctx)

	response := decodeApplicationAPIError(t, recorder)
	require.Contains(t, response.Message, "application header rules conflict")
	require.Contains(t, response.Message, application.Name)
}

func TestUpdateApplicationRejectsConflictingHeaderValidationRules(t *testing.T) {
	setupApplicationControllerTestDB(t)

	existing := &model.Application{
		AppKey: "existing-app",
		Name:   "Existing App",
		Status: 1,
	}
	require.NoError(t, existing.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
	}))
	require.NoError(t, existing.Insert())

	candidate := &model.Application{
		AppKey: "candidate-app",
		Name:   "Candidate App",
		Status: 1,
	}
	require.NoError(t, candidate.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://candidate.example.com"},
	}))
	require.NoError(t, candidate.Insert())

	ctx, recorder := newApplicationContext(t, http.MethodPut, "/api/application/"+strconv.FormatInt(candidate.Id, 10), dto.UpdateApplicationRequest{
		Name:      candidate.Name,
		Status:    1,
		SortOrder: 0,
		HeaderValidationRules: applicationControllerHeaderRulesPtr([]types.ApplicationHeaderValidationRule{
			{Header: "Origin", Operator: types.ApplicationHeaderOperatorOneOf, Values: []string{
				"https://candidate.example.com",
				"https://app.example.com",
			}},
		}),
	})
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(candidate.Id, 10)}}

	UpdateApplication(ctx)

	response := decodeApplicationAPIError(t, recorder)
	require.Contains(t, response.Message, "application header rules conflict")
	require.Contains(t, response.Message, existing.Name)
}

func TestUpdateApplicationStatusRejectsConflictingHeaderValidationRules(t *testing.T) {
	setupApplicationControllerTestDB(t)

	existing := &model.Application{
		AppKey: "existing-app",
		Name:   "Existing App",
		Status: 1,
	}
	require.NoError(t, existing.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
	}))
	require.NoError(t, existing.Insert())

	candidate := &model.Application{
		AppKey: "candidate-app",
		Name:   "Candidate App",
		Status: 0,
	}
	require.NoError(t, candidate.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorOneOf, Values: []string{
			"https://app.example.com",
			"https://other.example.com",
		}},
	}))
	require.NoError(t, candidate.Insert())

	ctx, recorder := newApplicationContext(t, http.MethodPatch, "/api/application/"+strconv.FormatInt(candidate.Id, 10)+"/status", map[string]any{
		"status": 1,
	})
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(candidate.Id, 10)}}

	UpdateApplicationStatus(ctx)

	response := decodeApplicationAPIError(t, recorder)
	require.Contains(t, response.Message, "application header rules conflict")
	require.Contains(t, response.Message, existing.Name)
}
