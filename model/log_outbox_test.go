package model

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRecordConsumeLogAddsRequestContextAndOutboxPayloadSeparately(t *testing.T) {
	DB.Exec("DELETE FROM log_outboxes")
	DB.Exec("DELETE FROM logs")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM log_outboxes")
		DB.Exec("DELETE FROM logs")
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("User-Agent", "unit-test")
	c.Request = req
	c.Set("username", "alice")
	c.Set(common.RequestIdKey, "req-1")
	c.Set(common.UpstreamRequestIdKey, "upstream-1")
	c.Set("token_name", "primary-token")
	c.Set("token_queue_priority", 8)
	c.Set("token_queue_timeout", 30)
	common.SetContextKey(c, constant.ContextKeyUserId, 42)
	common.SetContextKey(c, constant.ContextKeyUserName, "alice")
	common.SetContextKey(c, constant.ContextKeyUserDisplayName, "Alice")
	common.SetContextKey(c, constant.ContextKeyUserRole, common.RoleCommonUser)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserEmail, "alice@example.com")
	common.SetContextKey(c, constant.ContextKeyUserCompanyId, int64(7))
	common.SetContextKey(c, constant.ContextKeyUserCompanyName, "Acme")
	common.SetContextKey(c, constant.ContextKeyUserCompanyCode, "AC")
	common.SetContextKey(c, constant.ContextKeyApplicationId, 3)
	common.SetContextKey(c, constant.ContextKeyApplicationKey, "app-key")
	common.SetContextKey(c, constant.ContextKeyApplicationName, "Console")
	common.SetContextKey(c, constant.ContextKeyTokenId, 11)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(c, constant.ContextKeyChannelId, 5)
	common.SetContextKey(c, constant.ContextKeyChannelName, "openai-main")
	common.SetContextKey(c, constant.ContextKeyChannelType, 1)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	SetQueueLogSnapshot(c, RequestContextQueue{
		Required:                true,
		ModelName:               "gpt-test",
		PriorityEffective:       9,
		PriorityToken:           8,
		PriorityCompany:         6,
		TimeoutEffectiveSeconds: 30,
		PositionInitial:         2,
		WaitMs:                  15,
		Result:                  QueueResultAdmitted,
		EstimatedPromptTokens:   123,
	})
	SetUsageLogPayload(c, UsageLogPayload{
		ClientRequestHeadersJson: `{"Authorization":"Bearer secret","Content-Type":"application/json"}`,
		ClientRequestBody:        `{"password":"secret","message":"hello"}`,
		PayloadSizeBytes:         37,
		CaptureMode:              "full",
	})

	RecordConsumeLog(c, 42, RecordConsumeLogParams{
		ChannelId:        5,
		PromptTokens:     10,
		CompletionTokens: 20,
		ModelName:        "gpt-test",
		TokenName:        "primary-token",
		Quota:            100,
		Content:          "consume",
		TokenId:          11,
		UseTimeSeconds:   2,
		IsStream:         true,
		Group:            "default",
		Other: map[string]interface{}{
			"model_ratio":      2.5,
			"group_ratio":      1.2,
			"completion_ratio": 3.0,
			"billing_source":   "wallet",
		},
	})

	var log Log
	require.NoError(t, LOG_DB.First(&log).Error)
	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	require.Contains(t, other, "request_context")
	require.NotContains(t, other, "client_request_body")
	require.NotContains(t, other, "payload")

	var outbox LogOutbox
	require.NoError(t, LOG_DB.First(&outbox).Error)
	require.Equal(t, LogOutboxEventTypeUsage, outbox.EventType)
	require.Equal(t, LogOutboxStatusPending, outbox.KafkaStatus)
	require.Equal(t, LogOutboxStatusPending, outbox.ClickHouseStatus)

	var event CanonicalLogEvent
	require.NoError(t, common.UnmarshalJsonStr(string(outbox.PayloadJSON), &event))
	require.NotNil(t, event.Usage)
	require.Equal(t, "req-1", event.Usage.RequestContext.Request.RequestId)
	require.Equal(t, "Acme", event.Usage.RequestContext.Organization.CompanyName)
	require.Equal(t, QueueResultAdmitted, event.Usage.RequestContext.Queue.Result)
	require.NotNil(t, event.Usage.Payload)
	require.Contains(t, event.Usage.Payload.ClientRequestHeadersJson, "[REDACTED]")
	require.Contains(t, event.Usage.Payload.ClientRequestBody, "[REDACTED]")
}

func TestUsageLogPayloadRedactsPlainTextSensitiveFields(t *testing.T) {
	payload := sanitizeUsageLogPayload(UsageLogPayload{
		ClientRequestHeadersJson: `{"x-api-key":"provider-key","x-goog-api-key":"google-key","api-key":"azure-key","mj-api-secret":"mj-key"}`,
		ClientRequestBody:        "password=secret&message=hello&api-key=provider-key",
		ErrorBody:                "Authorization: Bearer secret-token\nx-api-key: provider-key\nok",
	})
	require.Contains(t, payload.ClientRequestHeadersJson, "[REDACTED]")
	require.NotContains(t, payload.ClientRequestHeadersJson, "provider-key")
	require.NotContains(t, payload.ClientRequestHeadersJson, "google-key")
	require.NotContains(t, payload.ClientRequestHeadersJson, "azure-key")
	require.NotContains(t, payload.ClientRequestHeadersJson, "mj-key")
	require.Contains(t, payload.ClientRequestBody, "password=[REDACTED]")
	require.NotContains(t, payload.ClientRequestBody, "secret")
	require.Contains(t, payload.ClientRequestBody, "api-key=[REDACTED]")
	require.Contains(t, payload.ErrorBody, "Authorization: [REDACTED]")
	require.Contains(t, payload.ErrorBody, "x-api-key: [REDACTED]")
	require.NotContains(t, payload.ErrorBody, "secret-token")

	truncatedJSON := redactJSONSensitiveFields(`{"api_key":"sk-secret","password":"p-secret","message":"truncated`)
	require.Contains(t, truncatedJSON, `"api_key":"[REDACTED]"`)
	require.Contains(t, truncatedJSON, `"password":"[REDACTED]"`)
	require.NotContains(t, truncatedJSON, "sk-secret")
	require.NotContains(t, truncatedJSON, "p-secret")
}

func TestRecordConsumeLogRespectsRecordIPSettingInRequestContext(t *testing.T) {
	DB.Exec("DELETE FROM log_outboxes")
	DB.Exec("DELETE FROM logs")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM log_outboxes")
		DB.Exec("DELETE FROM logs")
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.RemoteAddr = "192.0.2.10:12345"
	c.Request = req
	c.Set("username", "alice")
	common.SetContextKey(c, constant.ContextKeyUserId, 42)
	common.SetContextKey(c, constant.ContextKeyUserName, "alice")
	common.SetContextKey(c, constant.ContextKeyRecordIpLog, false)

	RecordConsumeLog(c, 42, RecordConsumeLogParams{
		ModelName: "gpt-test",
		Content:   "consume",
		Group:     "default",
	})

	var log Log
	require.NoError(t, LOG_DB.First(&log).Error)
	require.Empty(t, log.Ip)
	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	ctxBytes, err := common.Marshal(other["request_context"])
	require.NoError(t, err)
	var snapshot RequestContextSnapshot
	require.NoError(t, common.Unmarshal(ctxBytes, &snapshot))
	require.Empty(t, snapshot.Request.ClientIp)
}

func TestLogOutboxSinkStatusesAreIndependent(t *testing.T) {
	DB.Exec("DELETE FROM log_outboxes")
	DB.Exec("DELETE FROM logs")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM log_outboxes")
		DB.Exec("DELETE FROM logs")
	})

	log := &Log{
		UserId:    1,
		Username:  "alice",
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeConsume,
		Content:   "consume",
		ModelName: "gpt-test",
	}
	require.NoError(t, LOG_DB.Create(log).Error)
	require.NoError(t, EnqueueUsageLogOutbox(log, nil))

	var outbox LogOutbox
	require.NoError(t, LOG_DB.First(&outbox).Error)
	require.NoError(t, MarkLogOutboxSinkSent(outbox.Id, LogOutboxSinkKafka))
	require.NoError(t, MarkLogOutboxSinkFailed(outbox.Id, LogOutboxSinkClickHouse, 2, 0, "clickhouse down"))

	var updated LogOutbox
	require.NoError(t, LOG_DB.First(&updated, outbox.Id).Error)
	require.Equal(t, LogOutboxStatusSent, updated.KafkaStatus)
	require.Equal(t, 0, updated.KafkaRetryCount)
	require.Equal(t, LogOutboxStatusFailed, updated.ClickHouseStatus)
	require.Equal(t, 2, updated.ClickHouseRetryCount)
	require.Contains(t, updated.ClickHouseLastError, "clickhouse down")

	kafkaDue, err := ListLogOutboxDue(LogOutboxSinkKafka, common.GetTimestamp(), 10)
	require.NoError(t, err)
	require.Empty(t, kafkaDue)
	clickHouseDue, err := ListLogOutboxDue(LogOutboxSinkClickHouse, common.GetTimestamp(), 10)
	require.NoError(t, err)
	require.Len(t, clickHouseDue, 1)
}

func TestPayloadCaptureFeedsOutboxBeforeFinalize(t *testing.T) {
	DB.Exec("DELETE FROM log_outboxes")
	DB.Exec("DELETE FROM logs")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM log_outboxes")
		DB.Exec("DELETE FROM logs")
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"message":"hello"}`))
	c.Request.Header.Set("Authorization", "Bearer secret")
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("username", "alice")
	c.Set(common.RequestIdKey, "req-capture")
	common.SetContextKey(c, constant.ContextKeyUserId, 42)
	common.SetContextKey(c, constant.ContextKeyUserName, "alice")

	capture := StartUsageLogPayloadCapture(c)
	capture.CaptureClientRequest(c.Request)

	RecordConsumeLog(c, 42, RecordConsumeLogParams{
		ModelName: "gpt-test",
		TokenName: "primary-token",
		Quota:     1,
		Content:   "consume",
		Group:     "default",
	})

	var outbox LogOutbox
	require.NoError(t, LOG_DB.First(&outbox).Error)
	var event CanonicalLogEvent
	require.NoError(t, common.UnmarshalJsonStr(string(outbox.PayloadJSON), &event))
	require.NotNil(t, event.Usage)
	require.NotNil(t, event.Usage.Payload)
	require.Contains(t, event.Usage.Payload.ClientRequestHeadersJson, "[REDACTED]")
	require.Contains(t, event.Usage.Payload.ClientRequestBody, "hello")
}

func TestFinalizePayloadCaptureRefreshesOutboxResponseBody(t *testing.T) {
	DB.Exec("DELETE FROM log_outboxes")
	DB.Exec("DELETE FROM logs")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM log_outboxes")
		DB.Exec("DELETE FROM logs")
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"message":"hello"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("username", "alice")
	common.SetContextKey(c, constant.ContextKeyUserId, 42)
	common.SetContextKey(c, constant.ContextKeyUserName, "alice")

	capture := StartUsageLogPayloadCapture(c)
	capture.CaptureClientRequest(c.Request)
	RecordConsumeLog(c, 42, RecordConsumeLogParams{
		ModelName: "gpt-test",
		Content:   "consume",
		Group:     "default",
	})
	capture.CaptureClientResponseWrite([]byte(`{"choices":[{"message":{"content":"done"}}]}`))
	FinalizeUsageLogPayloadCapture(c)

	var outbox LogOutbox
	require.NoError(t, LOG_DB.First(&outbox).Error)
	var event CanonicalLogEvent
	require.NoError(t, common.UnmarshalJsonStr(string(outbox.PayloadJSON), &event))
	require.NotNil(t, event.Usage)
	require.NotNil(t, event.Usage.Payload)
	require.Contains(t, event.Usage.Payload.ClientResponseBody, "done")
	require.Greater(t, outbox.PayloadSizeBytes, 0)
}

func TestPayloadCaptureLimitsRequestBodyAndRestoresBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"message":"hello"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	capture := NewUsageLogPayloadCapture(5)
	capture.CaptureClientRequest(c.Request)

	payload := capture.Payload()
	require.Equal(t, `{"mes`, payload.ClientRequestBody)
	require.True(t, payload.Truncated)

	remaining, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, `{"message":"hello"}`, string(remaining))
}

func TestPayloadCaptureSkipsNonTextRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", strings.NewReader("binary-data"))
	c.Request.Header.Set("Content-Type", "image/png")

	capture := NewUsageLogPayloadCapture(5)
	capture.CaptureClientRequest(c.Request)

	payload := capture.Payload()
	require.Empty(t, payload.ClientRequestBody)
	require.False(t, payload.Truncated)

	remaining, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, "binary-data", string(remaining))
}

func TestPayloadCaptureModeDisabledByDefault(t *testing.T) {
	t.Setenv("USAGE_LOG_PAYLOAD_CAPTURE_ENABLED", "")
	require.Equal(t, "disabled", CaptureModeFromEnv())
}

func TestPayloadCaptureModeUsesTwoMiBDefault(t *testing.T) {
	t.Setenv("USAGE_LOG_PAYLOAD_CAPTURE_ENABLED", "true")
	t.Setenv("USAGE_LOG_PAYLOAD_CAPTURE_MAX_BYTES", "")
	require.Equal(t, "text:2097152", CaptureModeFromEnv())
}

func TestStripLogOutboxUsagePayloadKeepsMetrics(t *testing.T) {
	DB.Exec("DELETE FROM log_outboxes")
	DB.Exec("DELETE FROM logs")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM log_outboxes")
		DB.Exec("DELETE FROM logs")
	})

	log := &Log{
		UserId:    1,
		Username:  "alice",
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeConsume,
		Content:   "consume",
		Quota:     7,
	}
	require.NoError(t, LOG_DB.Create(log).Error)
	require.NoError(t, EnqueueUsageLogOutbox(log, &UsageLogPayload{
		ClientRequestBody: `{"message":"hello"}`,
		PayloadSizeBytes:  17,
		CaptureMode:       "full",
	}))

	var outbox LogOutbox
	require.NoError(t, LOG_DB.First(&outbox).Error)
	require.NoError(t, StripLogOutboxUsagePayload(outbox.Id))

	var updated LogOutbox
	require.NoError(t, LOG_DB.First(&updated, outbox.Id).Error)
	require.Equal(t, 7, updated.UsageQuota)
	require.Equal(t, 17, updated.PayloadSizeBytes)
	var event CanonicalLogEvent
	require.NoError(t, common.UnmarshalJsonStr(string(updated.PayloadJSON), &event))
	require.NotNil(t, event.Usage)
	require.Nil(t, event.Usage.Payload)
}

func TestClaimLogOutboxDueLeasesSinkIndependently(t *testing.T) {
	DB.Exec("DELETE FROM log_outboxes")
	DB.Exec("DELETE FROM logs")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM log_outboxes")
		DB.Exec("DELETE FROM logs")
	})

	log := &Log{
		UserId:    1,
		Username:  "alice",
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeConsume,
		Content:   "consume",
		ModelName: "gpt-test",
	}
	require.NoError(t, LOG_DB.Create(log).Error)
	require.NoError(t, EnqueueUsageLogOutbox(log, nil))

	now := common.GetTimestamp()
	claimed, err := ClaimLogOutboxDue(LogOutboxSinkKafka, now, 10, 60)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, LogOutboxStatusProcessing, claimed[0].KafkaStatus)

	claimedAgain, err := ClaimLogOutboxDue(LogOutboxSinkKafka, now, 10, 60)
	require.NoError(t, err)
	require.Empty(t, claimedAgain)

	clickHouseDue, err := ClaimLogOutboxDue(LogOutboxSinkClickHouse, now, 10, 60)
	require.NoError(t, err)
	require.Len(t, clickHouseDue, 1)
}

func TestRecordAuditLogEnqueuesAuditEvent(t *testing.T) {
	DB.Exec("DELETE FROM log_outboxes")
	DB.Exec("DELETE FROM logs")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM log_outboxes")
		DB.Exec("DELETE FROM logs")
	})

	err := RecordAuditLog(RecordAuditLogParams{
		ActorUserID:    9,
		ActorUsername:  "admin",
		ActorRole:      common.RoleAdminUser,
		TargetType:     "usage_log_payload",
		TargetID:       "usage-1",
		Action:         "payload.view",
		Result:         "success",
		Summary:        "view payload",
		RequestID:      "req-audit",
		RequestMethod:  http.MethodGet,
		RequestPath:    "/api/log/payload",
		ClientIP:       "127.0.0.1",
		ApplicationKey: "app-key",
		RequestContext: RequestContextSnapshot{
			User: RequestContextUser{
				Id:       9,
				Username: "admin",
				Role:     common.RoleAdminUser,
			},
		},
	})
	require.NoError(t, err)

	var outbox LogOutbox
	require.NoError(t, LOG_DB.First(&outbox).Error)
	require.Equal(t, LogOutboxEventTypeAudit, outbox.EventType)

	var event CanonicalLogEvent
	require.NoError(t, common.UnmarshalJsonStr(string(outbox.PayloadJSON), &event))
	require.NotNil(t, event.Audit)
	require.Equal(t, "payload.view", event.Audit.Action)
	require.Equal(t, "usage-1", event.Audit.TargetID)
	require.Equal(t, "app-key", event.Audit.ApplicationKey)
}

func TestRebuildMissingLogOutboxByLogIDRange(t *testing.T) {
	DB.Exec("DELETE FROM log_outboxes")
	DB.Exec("DELETE FROM logs")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM log_outboxes")
		DB.Exec("DELETE FROM logs")
	})

	log := &Log{
		UserId:    1,
		Username:  "alice",
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeConsume,
		Content:   "consume",
		Quota:     11,
	}
	require.NoError(t, LOG_DB.Create(log).Error)

	rebuilt, err := RebuildMissingLogOutboxByLogIDRange(log.Id, log.Id)
	require.NoError(t, err)
	require.EqualValues(t, 1, rebuilt)

	var outbox LogOutbox
	require.NoError(t, LOG_DB.Where("event_id = ?", stableLogEventID(LogOutboxEventTypeUsage, log.Id)).First(&outbox).Error)
	require.Equal(t, 11, outbox.UsageQuota)
	require.Equal(t, LogOutboxStatusPending, outbox.KafkaStatus)
	require.Equal(t, LogOutboxStatusPending, outbox.ClickHouseStatus)
}

func TestRecordTaskBillingLogPreservesRequestContext(t *testing.T) {
	DB.Exec("DELETE FROM log_outboxes")
	DB.Exec("DELETE FROM logs")
	DB.Exec("DELETE FROM users")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM log_outboxes")
		DB.Exec("DELETE FROM logs")
		DB.Exec("DELETE FROM users")
	})

	require.NoError(t, DB.Create(&User{Id: 77, Username: "task-user", Status: common.UserStatusEnabled}).Error)
	requestContext := &RequestContextSnapshot{
		User: RequestContextUser{
			Id:       77,
			Username: "task-user",
		},
		Application: RequestContextApplication{
			Id:   3,
			Key:  "app-key",
			Name: "Console",
		},
		Request: RequestContextRequest{
			RequestId: "req-task",
		},
	}
	RecordTaskBillingLog(RecordTaskBillingLogParams{
		UserId:         77,
		LogType:        LogTypeRefund,
		Content:        "refund",
		ModelName:      "gpt-test",
		Quota:          100,
		Group:          "default",
		RequestContext: requestContext,
	})

	var outbox LogOutbox
	require.NoError(t, LOG_DB.First(&outbox).Error)
	var event CanonicalLogEvent
	require.NoError(t, common.UnmarshalJsonStr(string(outbox.PayloadJSON), &event))
	require.NotNil(t, event.Usage)
	require.Equal(t, "app-key", event.Usage.RequestContext.Application.Key)
	require.Equal(t, "req-task", event.Usage.RequestContext.Request.RequestId)
}
