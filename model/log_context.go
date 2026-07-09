package model

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	QueueResultBypassed = "bypassed"
	QueueResultAdmitted = "admitted"
	QueueResultTimeout  = "timeout"
	QueueResultFull     = "full"
	QueueResultError    = "error"
)

type RequestContextSnapshot struct {
	User         RequestContextUser         `json:"user"`
	Organization RequestContextOrganization `json:"organization"`
	Application  RequestContextApplication  `json:"application"`
	Token        RequestContextToken        `json:"token"`
	Queue        RequestContextQueue        `json:"queue"`
	Request      RequestContextRequest      `json:"request"`
	Routing      RequestContextRouting      `json:"routing"`
	Billing      RequestContextBilling      `json:"billing"`
}

type RequestContextUser struct {
	Id          int    `json:"id"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Role        int    `json:"role,omitempty"`
	Group       string `json:"group,omitempty"`
	Email       string `json:"email,omitempty"`
}

type RequestContextOrganization struct {
	CompanyId           int64                     `json:"company_id,omitempty"`
	CompanyName         string                    `json:"company_name,omitempty"`
	CompanyCode         string                    `json:"company_code,omitempty"`
	DepartmentId        int64                     `json:"department_id,omitempty"`
	DepartmentName      string                    `json:"department_name,omitempty"`
	DepartmentPath      string                    `json:"department_path,omitempty"`
	DepartmentLevel     int                       `json:"department_level,omitempty"`
	DepartmentHierarchy []DepartmentHierarchyItem `json:"department_hierarchy,omitempty"`
}

type DepartmentHierarchyItem struct {
	Id    int64  `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}

type RequestContextApplication struct {
	Id              int                               `json:"id,omitempty"`
	Key             string                            `json:"key,omitempty"`
	Name            string                            `json:"name,omitempty"`
	HeaderDetection *types.ApplicationHeaderDetection `json:"header_detection,omitempty"`
}

type RequestContextToken struct {
	Id                  int    `json:"id,omitempty"`
	Name                string `json:"name,omitempty"`
	Group               string `json:"group,omitempty"`
	QueuePriority       int    `json:"queue_priority,omitempty"`
	QueueTimeoutSeconds int    `json:"queue_timeout_seconds,omitempty"`
	UnlimitedQuota      bool   `json:"unlimited_quota,omitempty"`
}

type RequestContextQueue struct {
	Required                bool   `json:"required"`
	ModelName               string `json:"model_name,omitempty"`
	PriorityEffective       int    `json:"priority_effective,omitempty"`
	PriorityToken           int    `json:"priority_token,omitempty"`
	PriorityCompany         int    `json:"priority_company,omitempty"`
	PriorityFormula         string `json:"priority_formula,omitempty"`
	PriorityRange           string `json:"priority_range,omitempty"`
	PriorityHigherIsFaster  bool   `json:"priority_higher_is_faster"`
	TimeoutEffectiveSeconds int    `json:"timeout_effective_seconds,omitempty"`
	PositionInitial         int    `json:"position_initial,omitempty"`
	WaitMs                  int64  `json:"wait_ms,omitempty"`
	Result                  string `json:"result,omitempty"`
	EstimatedPromptTokens   int    `json:"estimated_prompt_tokens,omitempty"`
	MatchedLongContextTier  int    `json:"matched_long_context_tier,omitempty"`
}

type RequestContextRequest struct {
	Method            string `json:"method,omitempty"`
	Path              string `json:"path,omitempty"`
	ClientIp          string `json:"client_ip,omitempty"`
	UserAgent         string `json:"user_agent,omitempty"`
	RequestId         string `json:"request_id,omitempty"`
	UpstreamRequestId string `json:"upstream_request_id,omitempty"`
	Stream            bool   `json:"stream"`
}

type RequestContextRouting struct {
	ModelName         string `json:"model_name,omitempty"`
	UpstreamModelName string `json:"upstream_model_name,omitempty"`
	ChannelId         int    `json:"channel_id,omitempty"`
	ChannelName       string `json:"channel_name,omitempty"`
	ChannelType       int    `json:"channel_type,omitempty"`
	IsMultiKey        bool   `json:"is_multi_key,omitempty"`
	MultiKeyIndex     int    `json:"multi_key_index,omitempty"`
	UsingGroup        string `json:"using_group,omitempty"`
}

type RequestContextBilling struct {
	BillingSource    string  `json:"billing_source,omitempty"`
	Quota            int     `json:"quota,omitempty"`
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	ModelRatio       float64 `json:"model_ratio,omitempty"`
	GroupRatio       float64 `json:"group_ratio,omitempty"`
	CompletionRatio  float64 `json:"completion_ratio,omitempty"`
	CacheTokens      int     `json:"cache_tokens,omitempty"`
	CacheRatio       float64 `json:"cache_ratio,omitempty"`
}

type UsageLogPayload struct {
	ClientRequestHeadersJson    string `json:"client_request_headers_json,omitempty"`
	ClientRequestBody           string `json:"client_request_body,omitempty"`
	UpstreamRequestHeadersJson  string `json:"upstream_request_headers_json,omitempty"`
	UpstreamRequestBody         string `json:"upstream_request_body,omitempty"`
	UpstreamResponseHeadersJson string `json:"upstream_response_headers_json,omitempty"`
	UpstreamResponseBody        string `json:"upstream_response_body,omitempty"`
	ClientResponseHeadersJson   string `json:"client_response_headers_json,omitempty"`
	ClientResponseBody          string `json:"client_response_body,omitempty"`
	ErrorBody                   string `json:"error_body,omitempty"`
	PayloadSizeBytes            int    `json:"payload_size_bytes,omitempty"`
	Truncated                   bool   `json:"truncated,omitempty"`
	CaptureMode                 string `json:"capture_mode,omitempty"`
}

func AttachRequestContextSnapshot(c *gin.Context) {
	if c == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyRequestContextSnapshot, BuildRequestContextSnapshot(c, nil, nil))
}

func SetQueueLogSnapshot(c *gin.Context, snapshot RequestContextQueue) {
	if c == nil {
		return
	}
	if snapshot.PriorityFormula == "" {
		snapshot.PriorityFormula = "token + company - 5"
	}
	if snapshot.PriorityRange == "" {
		snapshot.PriorityRange = "1..10"
	}
	snapshot.PriorityHigherIsFaster = true
	common.SetContextKey(c, constant.ContextKeyQueueSnapshot, snapshot)
	queueSnapshot := snapshot
	refreshRequestContextSnapshot(c, func(snapshot *RequestContextSnapshot) {
		snapshot.Queue = queueSnapshot
	})
}

func SetUsageLogPayload(c *gin.Context, payload UsageLogPayload) {
	if c == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyUsageLogPayload, sanitizeUsageLogPayload(payload))
}

func ShouldRecordRequestIP(c *gin.Context) bool {
	if c == nil {
		return false
	}
	if recordIP, ok := common.GetContextKeyType[bool](c, constant.ContextKeyRecordIpLog); ok {
		return recordIP
	}
	if setting, ok := common.GetContextKeyType[dto.UserSetting](c, constant.ContextKeyUserSetting); ok {
		return setting.RecordIpLog
	}
	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	if userID <= 0 {
		userID = c.GetInt("id")
	}
	if userID <= 0 {
		return false
	}
	if common.RedisEnabled && common.RDB == nil {
		return false
	}
	if DB == nil {
		return false
	}
	setting, err := GetUserSetting(userID, false)
	return err == nil && setting.RecordIpLog
}

func BuildRequestContextSnapshot(c *gin.Context, log *Log, other map[string]interface{}) RequestContextSnapshot {
	if c != nil {
		if snapshot, ok := common.GetContextKeyType[RequestContextSnapshot](c, constant.ContextKeyRequestContextSnapshot); ok {
			fillSnapshotFromLog(&snapshot, c, log, other)
			return snapshot
		}
	}
	snapshot := RequestContextSnapshot{}
	if c != nil {
		snapshot.User = RequestContextUser{
			Id:          common.GetContextKeyInt(c, constant.ContextKeyUserId),
			Username:    firstNonEmpty(common.GetContextKeyString(c, constant.ContextKeyUserName), c.GetString("username")),
			DisplayName: common.GetContextKeyString(c, constant.ContextKeyUserDisplayName),
			Role:        common.GetContextKeyInt(c, constant.ContextKeyUserRole),
			Group:       firstNonEmpty(common.GetContextKeyString(c, constant.ContextKeyUserGroup), c.GetString("user_group")),
			Email:       common.GetContextKeyString(c, constant.ContextKeyUserEmail),
		}
		companyID, _ := common.GetContextKeyType[int64](c, constant.ContextKeyUserCompanyId)
		departmentID, _ := common.GetContextKeyType[int64](c, constant.ContextKeyUserDepartmentId)
		snapshot.Organization = RequestContextOrganization{
			CompanyId:       companyID,
			CompanyName:     common.GetContextKeyString(c, constant.ContextKeyUserCompanyName),
			CompanyCode:     common.GetContextKeyString(c, constant.ContextKeyUserCompanyCode),
			DepartmentId:    departmentID,
			DepartmentName:  common.GetContextKeyString(c, constant.ContextKeyUserDepartmentName),
			DepartmentPath:  common.GetContextKeyString(c, constant.ContextKeyUserDepartmentPath),
			DepartmentLevel: common.GetContextKeyInt(c, constant.ContextKeyUserDepartmentLevel),
		}
		if raw := common.GetContextKeyString(c, constant.ContextKeyUserDepartmentHierarchy); raw != "" {
			_ = common.UnmarshalJsonStr(raw, &snapshot.Organization.DepartmentHierarchy)
		}
		snapshot.Application = RequestContextApplication{
			Id:              common.GetContextKeyInt(c, constant.ContextKeyApplicationId),
			Key:             common.GetContextKeyString(c, constant.ContextKeyApplicationKey),
			Name:            common.GetContextKeyString(c, constant.ContextKeyApplicationName),
			HeaderDetection: requestApplicationHeaderDetectionFromContext(c),
		}
		snapshot.Token = RequestContextToken{
			Id:                  common.GetContextKeyInt(c, constant.ContextKeyTokenId),
			Name:                c.GetString("token_name"),
			Group:               common.GetContextKeyString(c, constant.ContextKeyTokenGroup),
			QueuePriority:       c.GetInt("token_queue_priority"),
			QueueTimeoutSeconds: c.GetInt("token_queue_timeout"),
			UnlimitedQuota:      common.GetContextKeyBool(c, constant.ContextKeyTokenUnlimited),
		}
		if queue, ok := common.GetContextKeyType[RequestContextQueue](c, constant.ContextKeyQueueSnapshot); ok {
			snapshot.Queue = queue
		} else {
			snapshot.Queue = RequestContextQueue{
				Required:               common.GetContextKeyBool(c, constant.ContextKeyQueueRequired),
				ModelName:              common.GetContextKeyString(c, constant.ContextKeyQueueModelName),
				Result:                 QueueResultBypassed,
				PriorityFormula:        "token + company - 5",
				PriorityRange:          "1..10",
				PriorityHigherIsFaster: true,
				EstimatedPromptTokens:  common.GetContextKeyInt(c, constant.ContextKeyEstimatedTokens),
			}
		}
		if c.Request != nil {
			snapshot.Request = RequestContextRequest{
				Method:            c.Request.Method,
				Path:              requestPath(c.Request),
				ClientIp:          requestClientIPForLog(c, log),
				UserAgent:         c.Request.UserAgent(),
				RequestId:         c.GetString(common.RequestIdKey),
				UpstreamRequestId: c.GetString(common.UpstreamRequestIdKey),
				Stream:            common.GetContextKeyBool(c, constant.ContextKeyIsStream),
			}
		}
		snapshot.Routing = RequestContextRouting{
			ModelName:         common.GetContextKeyString(c, constant.ContextKeyQueueModelName),
			UpstreamModelName: stringFromMap(other, "upstream_model_name"),
			ChannelId:         common.GetContextKeyInt(c, constant.ContextKeyChannelId),
			ChannelName:       common.GetContextKeyString(c, constant.ContextKeyChannelName),
			ChannelType:       common.GetContextKeyInt(c, constant.ContextKeyChannelType),
			IsMultiKey:        common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey),
			MultiKeyIndex:     common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex),
			UsingGroup:        common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
		}
	}
	fillSnapshotFromLog(&snapshot, c, log, other)
	return snapshot
}

func MergeRequestContextIntoOther(c *gin.Context, log *Log, other map[string]interface{}) map[string]interface{} {
	if other == nil {
		other = make(map[string]interface{})
	}
	snapshot := BuildRequestContextSnapshot(c, log, other)
	other["request_context"] = snapshot
	return other
}

func GetUsageLogPayload(c *gin.Context) (UsageLogPayload, bool) {
	if c == nil {
		return UsageLogPayload{}, false
	}
	payload, ok := common.GetContextKeyType[UsageLogPayload](c, constant.ContextKeyUsageLogPayload)
	if ok {
		return sanitizeUsageLogPayload(payload), true
	}
	capture, ok := GetUsageLogPayloadCapture(c)
	if !ok || capture == nil {
		return UsageLogPayload{}, false
	}
	payload = capture.Payload()
	if payload.PayloadSizeBytes <= 0 && payload.CaptureMode == "" {
		return UsageLogPayload{}, false
	}
	return sanitizeUsageLogPayload(payload), true
}

func refreshRequestContextSnapshot(c *gin.Context, apply func(*RequestContextSnapshot)) {
	if c == nil || apply == nil {
		return
	}
	snapshot := BuildRequestContextSnapshot(c, nil, nil)
	apply(&snapshot)
	common.SetContextKey(c, constant.ContextKeyRequestContextSnapshot, snapshot)
}

func fillSnapshotFromLog(snapshot *RequestContextSnapshot, c *gin.Context, log *Log, other map[string]interface{}) {
	if snapshot == nil {
		return
	}
	if log != nil {
		if snapshot.User.Id == 0 {
			snapshot.User.Id = log.UserId
		}
		if snapshot.User.Username == "" {
			snapshot.User.Username = log.Username
		}
		if snapshot.Token.Id == 0 {
			snapshot.Token.Id = log.TokenId
		}
		if snapshot.Token.Name == "" {
			snapshot.Token.Name = log.TokenName
		}
		if snapshot.Routing.ModelName == "" {
			snapshot.Routing.ModelName = log.ModelName
		}
		if snapshot.Routing.ChannelId == 0 {
			snapshot.Routing.ChannelId = log.ChannelId
		}
		if snapshot.Routing.UsingGroup == "" {
			snapshot.Routing.UsingGroup = log.Group
		}
		if snapshot.Request.RequestId == "" {
			snapshot.Request.RequestId = log.RequestId
		}
		if snapshot.Request.UpstreamRequestId == "" {
			snapshot.Request.UpstreamRequestId = log.UpstreamRequestId
		}
		if snapshot.Request.ClientIp == "" {
			snapshot.Request.ClientIp = log.Ip
		}
		snapshot.Request.Stream = log.IsStream
		snapshot.Billing.Quota = log.Quota
		snapshot.Billing.PromptTokens = log.PromptTokens
		snapshot.Billing.CompletionTokens = log.CompletionTokens
	}
	if other != nil {
		if snapshot.Request.Path == "" {
			snapshot.Request.Path = stringFromMap(other, "request_path")
		}
		if snapshot.Routing.UpstreamModelName == "" {
			snapshot.Routing.UpstreamModelName = stringFromMap(other, "upstream_model_name")
		}
		snapshot.Billing.BillingSource = stringFromMap(other, "billing_source")
		snapshot.Billing.ModelRatio = floatFromMap(other, "model_ratio")
		snapshot.Billing.GroupRatio = floatFromMap(other, "group_ratio")
		snapshot.Billing.CompletionRatio = floatFromMap(other, "completion_ratio")
		snapshot.Billing.CacheTokens = intFromMap(other, "cache_tokens")
		snapshot.Billing.CacheRatio = floatFromMap(other, "cache_ratio")
	}
	if c != nil && snapshot.Request.Path == "" && c.Request != nil {
		snapshot.Request.Path = requestPath(c.Request)
	}
	if c != nil {
		if snapshot.Routing.ChannelId == 0 {
			snapshot.Routing.ChannelId = common.GetContextKeyInt(c, constant.ContextKeyChannelId)
		}
		if snapshot.Routing.ChannelName == "" {
			snapshot.Routing.ChannelName = common.GetContextKeyString(c, constant.ContextKeyChannelName)
		}
		if snapshot.Routing.ChannelType == 0 {
			snapshot.Routing.ChannelType = common.GetContextKeyInt(c, constant.ContextKeyChannelType)
		}
		if !snapshot.Routing.IsMultiKey {
			snapshot.Routing.IsMultiKey = common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey)
		}
		if snapshot.Routing.MultiKeyIndex == 0 {
			snapshot.Routing.MultiKeyIndex = common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
		}
		if snapshot.Routing.UsingGroup == "" {
			snapshot.Routing.UsingGroup = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
		}
		if snapshot.Application.Id == 0 {
			snapshot.Application.Id = common.GetContextKeyInt(c, constant.ContextKeyApplicationId)
		}
		if snapshot.Application.Key == "" {
			snapshot.Application.Key = common.GetContextKeyString(c, constant.ContextKeyApplicationKey)
		}
		if snapshot.Application.Name == "" {
			snapshot.Application.Name = common.GetContextKeyString(c, constant.ContextKeyApplicationName)
		}
		if snapshot.Application.HeaderDetection == nil {
			snapshot.Application.HeaderDetection = requestApplicationHeaderDetectionFromContext(c)
		}
	}
}

func requestApplicationHeaderDetectionFromContext(c *gin.Context) *types.ApplicationHeaderDetection {
	if c == nil {
		return nil
	}
	detection, ok := common.GetContextKeyType[types.ApplicationHeaderDetection](c, constant.ContextKeyApplicationHeaderDetection)
	if !ok {
		return nil
	}
	return &detection
}

func requestClientIPForLog(c *gin.Context, log *Log) string {
	if log != nil && log.Ip != "" {
		return log.Ip
	}
	if c != nil && ShouldRecordRequestIP(c) {
		return c.ClientIP()
	}
	return ""
}

func requestPath(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	return r.URL.Path
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func stringFromMap(data map[string]interface{}, key string) string {
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func intFromMap(data map[string]interface{}, key string) int {
	value, ok := data[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func floatFromMap(data map[string]interface{}, key string) float64 {
	value, ok := data[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}

func sanitizeUsageLogPayload(payload UsageLogPayload) UsageLogPayload {
	payload.ClientRequestHeadersJson = sanitizeHeaderJSON(payload.ClientRequestHeadersJson)
	payload.UpstreamRequestHeadersJson = sanitizeHeaderJSON(payload.UpstreamRequestHeadersJson)
	payload.UpstreamResponseHeadersJson = sanitizeHeaderJSON(payload.UpstreamResponseHeadersJson)
	payload.ClientResponseHeadersJson = sanitizeHeaderJSON(payload.ClientResponseHeadersJson)
	payload.ClientRequestBody = redactJSONSensitiveFields(payload.ClientRequestBody)
	payload.UpstreamRequestBody = redactJSONSensitiveFields(payload.UpstreamRequestBody)
	payload.UpstreamResponseBody = redactJSONSensitiveFields(payload.UpstreamResponseBody)
	payload.ClientResponseBody = redactJSONSensitiveFields(payload.ClientResponseBody)
	payload.ErrorBody = redactJSONSensitiveFields(payload.ErrorBody)
	return payload
}

func sanitizeHeaderJSON(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	var data map[string]interface{}
	if err := common.UnmarshalJsonStr(raw, &data); err != nil {
		return raw
	}
	redactSensitiveMap(data)
	bytes, err := common.Marshal(data)
	if err != nil {
		return raw
	}
	return string(bytes)
}

func redactJSONSensitiveFields(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	var data interface{}
	if err := common.UnmarshalJsonStr(raw, &data); err != nil {
		return redactPlainTextSensitiveFields(raw)
	}
	redactSensitiveValue(data)
	bytes, err := common.Marshal(data)
	if err != nil {
		return raw
	}
	return string(bytes)
}

var (
	sensitivePayloadKeyPattern      = `authorization|proxy-authorization|cookie|set-cookie|x-api-key|x-goog-api-key|api-key|anthropic-api-key|mj-api-secret|password|token_key|channel_key|oauth_secret|client_secret|api_secret|api-secret|api_key|api-key|payment_secret|stripe_api_secret`
	sensitiveQuotedJSONValuePattern = regexp.MustCompile(`(?i)(["'])(` + sensitivePayloadKeyPattern + `)(["']\s*:\s*)(["'])([^"'\r\n]*)`)
	sensitiveQuotedJSONBarePattern  = regexp.MustCompile(`(?i)(["'])(` + sensitivePayloadKeyPattern + `)(["']\s*:\s*)([^"',}\]\s]+)`)
	sensitiveHeaderTextPattern      = regexp.MustCompile(`(?i)\b(authorization|proxy-authorization|cookie|set-cookie|x-api-key|x-goog-api-key|api-key|anthropic-api-key|mj-api-secret)(\s*[:=]\s*)([^\r\n]+)`)
	sensitiveKVTextPattern          = regexp.MustCompile(`(?i)\b(password|token_key|channel_key|oauth_secret|client_secret|api_secret|api-secret|api_key|api-key|payment_secret|stripe_api_secret|mj-api-secret)(\s*[:=]\s*)([^&\s,;}]+)`)
)

func redactPlainTextSensitiveFields(raw string) string {
	redacted := sensitiveQuotedJSONValuePattern.ReplaceAllString(raw, `${1}${2}${3}${4}[REDACTED]`)
	redacted = sensitiveQuotedJSONBarePattern.ReplaceAllString(redacted, `${1}${2}${3}[REDACTED]`)
	redacted = sensitiveHeaderTextPattern.ReplaceAllString(redacted, `${1}${2}[REDACTED]`)
	redacted = sensitiveKVTextPattern.ReplaceAllString(redacted, `${1}${2}[REDACTED]`)
	return redacted
}

func redactSensitiveValue(value interface{}) {
	switch v := value.(type) {
	case map[string]interface{}:
		redactSensitiveMap(v)
	case []interface{}:
		for _, item := range v {
			redactSensitiveValue(item)
		}
	}
}

func redactSensitiveMap(data map[string]interface{}) {
	for key, value := range data {
		if isSensitivePayloadKey(key) {
			data[key] = "[REDACTED]"
			continue
		}
		redactSensitiveValue(value)
	}
}

func isSensitivePayloadKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	sensitiveKeys := []string{
		"authorization",
		"cookie",
		"set-cookie",
		"proxy-authorization",
		"x-api-key",
		"x-goog-api-key",
		"api-key",
		"api_key",
		"apikey",
		"anthropic-api-key",
		"mj-api-secret",
		"password",
		"token_key",
		"channel_key",
		"oauth_secret",
		"client_secret",
		"api_secret",
		"api-secret",
		"payment_secret",
		"stripe_api_secret",
	}
	for _, sensitiveKey := range sensitiveKeys {
		if normalized == sensitiveKey || strings.Contains(normalized, sensitiveKey) {
			return true
		}
	}
	if strings.HasSuffix(normalized, "-api-key") || strings.HasSuffix(normalized, "_api_key") ||
		strings.HasSuffix(normalized, "-api-secret") || strings.HasSuffix(normalized, "_api_secret") {
		return true
	}
	return false
}
