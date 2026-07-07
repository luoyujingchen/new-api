package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUserAuthAcceptsSessionWithoutUserHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("auth-test"))))
	router.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "tester")
		session.Set("role", common.RoleCommonUser)
		session.Set("id", 1)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	router.GET("/api/test", UserAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodGet, "/login", nil)
	router.ServeHTTP(loginRecorder, loginRequest)
	require.Equal(t, http.StatusNoContent, loginRecorder.Code)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	for _, cookie := range loginRecorder.Result().Cookies() {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func setupApplicationHeaderAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	require.NoError(t, db.AutoMigrate(&model.Application{}, &model.Log{}, &model.LogOutbox{}))

	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
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

func TestSetupContextForRequestApplicationRejectsUnregisteredHeaderAndRecordsErrorLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupApplicationHeaderAuthTestDB(t)

	application := &model.Application{
		AppKey: "registered-app",
		Name:   "Registered App",
		Status: 1,
	}
	require.NoError(t, application.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://registered.example.com"},
	}))
	require.NoError(t, application.Insert())

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Origin", "https://unknown.example.com")
	c.Set("username", "alice")
	common.SetContextKey(c, constant.ContextKeyUserName, "alice")
	common.SetContextKey(c, constant.ContextKeyRecordIpLog, false)

	token := &model.Token{
		Id:     7,
		UserId: 42,
		Name:   "application-test-token",
		Group:  "default",
	}

	err := SetupContextForRequestApplication(c, token)
	require.ErrorIs(t, err, service.ErrApplicationUnrecognized)
	require.Equal(t, http.StatusForbidden, recorder.Code)

	var logs []model.Log
	require.NoError(t, db.Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, model.LogTypeError, logs[0].Type)
	require.Equal(t, token.Id, logs[0].TokenId)
	require.Contains(t, logs[0].Content, "application header validation rejected")

	var other map[string]interface{}
	require.NoError(t, common.Unmarshal([]byte(logs[0].Other), &other))
	require.Equal(t, "unregistered_application", other["reject_reason"])
	if _, ok := other["application_header_validation"]; !ok {
		require.Fail(t, "expected application_header_validation in error log")
	}
}

func TestSetupContextForRequestApplicationRejectsAmbiguousHeaderAndRecordsErrorLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupApplicationHeaderAuthTestDB(t)

	equalsApplication := &model.Application{
		AppKey: "equals-app",
		Name:   "Equals App",
		Status: 1,
	}
	require.NoError(t, equalsApplication.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://app.example.com"},
	}))
	require.NoError(t, equalsApplication.Insert())

	oneOfApplication := &model.Application{
		AppKey: "one-of-app",
		Name:   "One Of App",
		Status: 1,
	}
	require.NoError(t, oneOfApplication.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorOneOf, Values: []string{
			"https://app.example.com",
			"https://other.example.com",
		}},
	}))
	require.NoError(t, oneOfApplication.Insert())

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Origin", "https://app.example.com")
	common.SetContextKey(c, constant.ContextKeyRecordIpLog, false)

	token := &model.Token{
		Id:     9,
		UserId: 44,
		Name:   "ambiguous-application-test-token",
		Group:  "default",
	}

	err := SetupContextForRequestApplication(c, token)
	require.ErrorIs(t, err, service.ErrApplicationHeaderAmbiguous)
	require.Equal(t, http.StatusForbidden, recorder.Code)

	var logs []model.Log
	require.NoError(t, db.Find(&logs).Error)
	require.Len(t, logs, 1)

	var other map[string]interface{}
	require.NoError(t, common.Unmarshal([]byte(logs[0].Other), &other))
	require.Equal(t, "application_header_ambiguous", other["reject_reason"])
}

func TestSetupContextForRequestApplicationRejectsBoundTokenHeaderMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupApplicationHeaderAuthTestDB(t)

	boundApplication := &model.Application{
		AppKey: "bound-app",
		Name:   "Bound App",
		Status: 1,
	}
	require.NoError(t, boundApplication.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://bound.example.com"},
	}))
	require.NoError(t, boundApplication.Insert())

	headerApplication := &model.Application{
		AppKey: "header-app",
		Name:   "Header App",
		Status: 1,
	}
	require.NoError(t, headerApplication.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://header.example.com"},
	}))
	require.NoError(t, headerApplication.Insert())

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Origin", "https://header.example.com")
	common.SetContextKey(c, constant.ContextKeyRecordIpLog, false)

	token := &model.Token{
		Id:            10,
		UserId:        45,
		Name:          "bound-mismatch-token",
		Group:         "default",
		ApplicationId: &boundApplication.Id,
	}

	err := SetupContextForRequestApplication(c, token)
	require.ErrorIs(t, err, service.ErrTokenApplicationMismatch)
	require.Equal(t, http.StatusForbidden, recorder.Code)

	var logs []model.Log
	require.NoError(t, db.Find(&logs).Error)
	require.Len(t, logs, 1)

	var other map[string]interface{}
	require.NoError(t, common.Unmarshal([]byte(logs[0].Other), &other))
	require.Equal(t, "application_header_token_mismatch", other["reject_reason"])
}

func TestSetupContextForRequestApplicationAllowsMatchingXAppIdAndHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupApplicationHeaderAuthTestDB(t)

	application := &model.Application{
		AppKey: "matched-app",
		Name:   "Matched App",
		Status: 1,
	}
	require.NoError(t, application.SetHeaderValidationRules([]types.ApplicationHeaderValidationRule{
		{Header: "Origin", Operator: types.ApplicationHeaderOperatorEquals, Value: "https://matched.example.com"},
	}))
	require.NoError(t, application.Insert())

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Origin", "https://matched.example.com")
	c.Request.Header.Set("X-App-Id", application.AppKey)
	token := &model.Token{
		Id:            11,
		UserId:        46,
		Name:          "matching-x-app-id-token",
		Group:         "default",
		ApplicationId: &application.Id,
	}

	err := SetupContextForRequestApplication(c, token)
	require.NoError(t, err)
	require.False(t, c.IsAborted())
	require.Equal(t, "", c.Request.Header.Get("X-App-Id"))
	require.Equal(t, int(application.Id), common.GetContextKeyInt(c, constant.ContextKeyApplicationId))
	require.Equal(t, application.AppKey, common.GetContextKeyString(c, constant.ContextKeyApplicationKey))
	require.Equal(t, application.Name, common.GetContextKeyString(c, constant.ContextKeyApplicationName))
}

func TestSetupContextForRequestApplicationKeepsLegacyBehaviorWithoutHeaderRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupApplicationHeaderAuthTestDB(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	token := &model.Token{Id: 8, UserId: 43, Name: "legacy-token", Group: "default"}

	err := SetupContextForRequestApplication(c, token)
	require.NoError(t, err)
	require.False(t, c.IsAborted())
	require.True(t, recorder.Code == http.StatusOK || recorder.Code == 0)
	require.False(t, errors.Is(err, service.ErrApplicationUnrecognized))
}
