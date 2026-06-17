package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/segmentio/kafka-go"
)

const (
	defaultUsageKafkaTopic = "newapi.usage_logs.v1"
	defaultAuditKafkaTopic = "newapi.audit_logs.v1"
)

type logOutboxSink interface {
	Name() string
	Publish(ctx context.Context, events []model.LogOutbox) error
}

type logOutboxPublisher struct {
	batchSize  int
	interval   time.Duration
	maxBackoff time.Duration
	lease      time.Duration
	sinks      []logOutboxSink
}

var startLogOutboxPublisherOnce sync.Once

func StartLogOutboxPublisher() {
	startLogOutboxPublisherOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		if !common.GetEnvOrDefaultBool("LOG_OUTBOX_PUBLISHER_ENABLED", true) {
			common.SysLog("log outbox publisher disabled")
			return
		}
		publisher := newLogOutboxPublisherFromEnv()
		if len(publisher.sinks) == 0 {
			common.SysLog("log outbox publisher has no configured sinks")
			return
		}
		go publisher.run(context.Background())
		common.SysLog("log outbox publisher started")
	})
}

func newLogOutboxPublisherFromEnv() *logOutboxPublisher {
	batchSize := common.GetEnvOrDefault("LOG_OUTBOX_BATCH_SIZE", 100)
	if batchSize <= 0 {
		batchSize = 100
	}
	intervalSeconds := common.GetEnvOrDefault("LOG_OUTBOX_PUBLISH_INTERVAL_SECONDS", 5)
	if intervalSeconds <= 0 {
		intervalSeconds = 5
	}
	maxBackoffSeconds := common.GetEnvOrDefault("LOG_OUTBOX_MAX_BACKOFF_SECONDS", 300)
	if maxBackoffSeconds <= 0 {
		maxBackoffSeconds = 300
	}
	leaseSeconds := common.GetEnvOrDefault("LOG_OUTBOX_PROCESSING_LEASE_SECONDS", 300)
	if leaseSeconds <= 0 {
		leaseSeconds = 300
	}
	publisher := &logOutboxPublisher{
		batchSize:  batchSize,
		interval:   time.Duration(intervalSeconds) * time.Second,
		maxBackoff: time.Duration(maxBackoffSeconds) * time.Second,
		lease:      time.Duration(leaseSeconds) * time.Second,
	}
	if sink := newKafkaNativeSinkFromEnv(); sink != nil {
		publisher.sinks = append(publisher.sinks, sink)
	} else if sink := newKafkaRESTSinkFromEnv(); sink != nil {
		publisher.sinks = append(publisher.sinks, sink)
	}
	if sink := newClickHouseSinkFromEnv(); sink != nil {
		if err := sink.EnsureSchema(context.Background()); err != nil {
			common.SysError("failed to ensure ClickHouse log schema: " + err.Error())
		}
		publisher.sinks = append(publisher.sinks, sink)
	}
	return publisher
}

func (p *logOutboxPublisher) run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		p.publishOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *logOutboxPublisher) publishOnce(ctx context.Context) {
	now := common.GetTimestamp()
	for _, sink := range p.sinks {
		events, err := model.ClaimLogOutboxDue(sink.Name(), now, p.batchSize, int64(p.lease.Seconds()))
		if err != nil {
			common.SysError(fmt.Sprintf("failed to list log outbox events for %s: %v", sink.Name(), err))
			continue
		}
		if len(events) == 0 {
			continue
		}
		if err := sink.Publish(ctx, events); err != nil {
			for _, event := range events {
				retryCount := logOutboxRetryCount(event, sink.Name()) + 1
				nextRetryAt := time.Now().Add(p.backoff(retryCount)).Unix()
				if markErr := model.MarkLogOutboxSinkFailed(event.Id, sink.Name(), retryCount, nextRetryAt, err.Error()); markErr != nil {
					common.SysError(fmt.Sprintf("failed to mark log outbox event %d failed for %s: %v", event.Id, sink.Name(), markErr))
				}
			}
			common.SysError(fmt.Sprintf("failed to publish %d log outbox events to %s: %v", len(events), sink.Name(), err))
			continue
		}
		for _, event := range events {
			if err := model.MarkLogOutboxSinkSent(event.Id, sink.Name()); err != nil {
				common.SysError(fmt.Sprintf("failed to mark log outbox event %d sent for %s: %v", event.Id, sink.Name(), err))
			}
			if sink.Name() == model.LogOutboxSinkClickHouse {
				if err := model.StripLogOutboxUsagePayload(event.Id); err != nil {
					common.SysError(fmt.Sprintf("failed to strip usage payload from log outbox event %d: %v", event.Id, err))
				}
			}
		}
	}
}

func (p *logOutboxPublisher) backoff(retryCount int) time.Duration {
	if retryCount <= 0 {
		return p.interval
	}
	seconds := 1 << min(retryCount-1, 10)
	backoff := time.Duration(seconds) * time.Second
	if backoff > p.maxBackoff {
		return p.maxBackoff
	}
	return backoff
}

func logOutboxRetryCount(event model.LogOutbox, sink string) int {
	switch sink {
	case model.LogOutboxSinkKafka:
		return event.KafkaRetryCount
	case model.LogOutboxSinkClickHouse:
		return event.ClickHouseRetryCount
	default:
		return 0
	}
}

type kafkaRESTSink struct {
	baseURL    string
	username   string
	password   string
	usageTopic string
	auditTopic string
	client     *http.Client
}

type kafkaNativeSink struct {
	brokers    []string
	usageTopic string
	auditTopic string
}

func newKafkaNativeSinkFromEnv() *kafkaNativeSink {
	rawBrokers := strings.TrimSpace(os.Getenv("LOG_OUTBOX_KAFKA_BROKERS"))
	if rawBrokers == "" {
		return nil
	}
	parts := strings.Split(rawBrokers, ",")
	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			brokers = append(brokers, part)
		}
	}
	if len(brokers) == 0 {
		return nil
	}
	return &kafkaNativeSink{
		brokers:    brokers,
		usageTopic: common.GetEnvOrDefaultString("LOG_OUTBOX_KAFKA_USAGE_TOPIC", defaultUsageKafkaTopic),
		auditTopic: common.GetEnvOrDefaultString("LOG_OUTBOX_KAFKA_AUDIT_TOPIC", defaultAuditKafkaTopic),
	}
}

func (s *kafkaNativeSink) Name() string {
	return model.LogOutboxSinkKafka
}

func (s *kafkaNativeSink) Publish(ctx context.Context, events []model.LogOutbox) error {
	byTopic := make(map[string][]model.LogOutbox)
	for _, event := range events {
		switch event.EventType {
		case model.LogOutboxEventTypeUsage:
			byTopic[s.usageTopic] = append(byTopic[s.usageTopic], event)
		case model.LogOutboxEventTypeAudit:
			byTopic[s.auditTopic] = append(byTopic[s.auditTopic], event)
		}
	}
	for topic, topicEvents := range byTopic {
		if len(topicEvents) == 0 {
			continue
		}
		writer := &kafka.Writer{
			Addr:         kafka.TCP(s.brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
		}
		messages := make([]kafka.Message, 0, len(topicEvents))
		for _, event := range topicEvents {
			payload, err := kafkaEventPayload(event)
			if err != nil {
				return err
			}
			messages = append(messages, kafka.Message{
				Key:   []byte(event.EventID),
				Value: payload,
				Time:  time.Now(),
			})
		}
		err := writer.WriteMessages(ctx, messages...)
		closeErr := writer.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func newKafkaRESTSinkFromEnv() *kafkaRESTSink {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("LOG_OUTBOX_KAFKA_REST_URL")), "/")
	if baseURL == "" {
		return nil
	}
	timeoutSeconds := common.GetEnvOrDefault("LOG_OUTBOX_KAFKA_TIMEOUT_SECONDS", 10)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}
	return &kafkaRESTSink{
		baseURL:    baseURL,
		username:   os.Getenv("LOG_OUTBOX_KAFKA_REST_USERNAME"),
		password:   os.Getenv("LOG_OUTBOX_KAFKA_REST_PASSWORD"),
		usageTopic: common.GetEnvOrDefaultString("LOG_OUTBOX_KAFKA_USAGE_TOPIC", defaultUsageKafkaTopic),
		auditTopic: common.GetEnvOrDefaultString("LOG_OUTBOX_KAFKA_AUDIT_TOPIC", defaultAuditKafkaTopic),
		client:     &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
	}
}

func (s *kafkaRESTSink) Name() string {
	return model.LogOutboxSinkKafka
}

func (s *kafkaRESTSink) Publish(ctx context.Context, events []model.LogOutbox) error {
	byTopic := make(map[string][]model.LogOutbox)
	for _, event := range events {
		switch event.EventType {
		case model.LogOutboxEventTypeUsage:
			byTopic[s.usageTopic] = append(byTopic[s.usageTopic], event)
		case model.LogOutboxEventTypeAudit:
			byTopic[s.auditTopic] = append(byTopic[s.auditTopic], event)
		}
	}
	for topic, topicEvents := range byTopic {
		if len(topicEvents) == 0 {
			continue
		}
		if err := s.publishTopic(ctx, topic, topicEvents); err != nil {
			return err
		}
	}
	return nil
}

func (s *kafkaRESTSink) publishTopic(ctx context.Context, topic string, events []model.LogOutbox) error {
	records := make([]map[string]interface{}, 0, len(events))
	for _, event := range events {
		var value interface{}
		payload, err := kafkaEventPayload(event)
		if err != nil {
			return err
		}
		if err := common.Unmarshal(payload, &value); err != nil {
			return err
		}
		records = append(records, map[string]interface{}{
			"key":   event.EventID,
			"value": value,
		})
	}
	bodyBytes, err := common.Marshal(map[string]interface{}{"records": records})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/topics/"+url.PathEscape(topic), bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/vnd.kafka.json.v2+json")
	req.Header.Set("Accept", "application/vnd.kafka.v2+json")
	if s.username != "" || s.password != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("kafka REST publish failed with status %d", resp.StatusCode)
	}
	return nil
}

func kafkaEventPayload(outbox model.LogOutbox) ([]byte, error) {
	var event model.CanonicalLogEvent
	if err := common.UnmarshalJsonStr(string(outbox.PayloadJSON), &event); err != nil {
		return nil, err
	}
	if event.Usage != nil {
		event.Usage.Payload = nil
	}
	return common.Marshal(event)
}

type clickHouseSink struct {
	baseURL  string
	database string
	username string
	password string
	ttlDays  int
	client   *http.Client
}

func newClickHouseSinkFromEnv() *clickHouseSink {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("LOG_OUTBOX_CLICKHOUSE_URL")), "/")
	if baseURL == "" {
		return nil
	}
	timeoutSeconds := common.GetEnvOrDefault("LOG_OUTBOX_CLICKHOUSE_TIMEOUT_SECONDS", 15)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 15
	}
	ttlDays := common.GetEnvOrDefault("LOG_OUTBOX_CLICKHOUSE_TTL_DAYS", 180)
	if ttlDays <= 0 {
		ttlDays = 180
	}
	return &clickHouseSink{
		baseURL:  baseURL,
		database: os.Getenv("LOG_OUTBOX_CLICKHOUSE_DATABASE"),
		username: os.Getenv("LOG_OUTBOX_CLICKHOUSE_USERNAME"),
		password: os.Getenv("LOG_OUTBOX_CLICKHOUSE_PASSWORD"),
		ttlDays:  ttlDays,
		client:   &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
	}
}

func (s *clickHouseSink) Name() string {
	return model.LogOutboxSinkClickHouse
}

func (s *clickHouseSink) EnsureSchema(ctx context.Context) error {
	queries := []string{
		s.usageLogsDDL(),
		s.usageLogPayloadsDDL(),
		s.auditLogsDDL(),
	}
	for _, query := range queries {
		if err := s.postQuery(ctx, query, nil); err != nil {
			return err
		}
	}
	return nil
}

func (s *clickHouseSink) Publish(ctx context.Context, events []model.LogOutbox) error {
	usageRows := make([]clickHouseUsageLogRow, 0, len(events))
	payloadRows := make([]clickHouseUsageLogPayloadRow, 0)
	auditRows := make([]clickHouseAuditLogRow, 0, len(events))
	for _, outbox := range events {
		var event model.CanonicalLogEvent
		if err := common.UnmarshalJsonStr(string(outbox.PayloadJSON), &event); err != nil {
			return err
		}
		if event.Usage != nil {
			usageRows = append(usageRows, buildClickHouseUsageLogRow(event.Usage))
			if event.Usage.Payload != nil {
				payloadRows = append(payloadRows, buildClickHouseUsageLogPayloadRow(event.Usage))
			}
		}
		if event.Audit != nil {
			auditRows = append(auditRows, buildClickHouseAuditLogRow(event.Audit))
		}
	}
	if len(payloadRows) > 0 {
		if err := s.insertJSONEachRow(ctx, "usage_log_payloads", payloadRows); err != nil {
			return err
		}
	}
	if len(auditRows) > 0 {
		if err := s.insertJSONEachRow(ctx, "audit_logs", auditRows); err != nil {
			return err
		}
	}
	if len(usageRows) > 0 {
		if err := s.insertJSONEachRow(ctx, "usage_logs", usageRows); err != nil {
			return err
		}
	}
	return nil
}

func (s *clickHouseSink) insertJSONEachRow(ctx context.Context, table string, rows interface{}) error {
	body, err := jsonEachRow(rows)
	if err != nil {
		return err
	}
	return s.postQuery(ctx, "INSERT INTO "+table+" FORMAT JSONEachRow", []byte(body))
}

func (s *clickHouseSink) postQuery(ctx context.Context, query string, body []byte) error {
	endpoint, err := url.Parse(s.baseURL + "/")
	if err != nil {
		return err
	}
	values := endpoint.Query()
	values.Set("query", query)
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "INSERT ") {
		values.Set("insert_deduplicate", "1")
	}
	if s.database != "" {
		values.Set("database", s.database)
	}
	endpoint.RawQuery = values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.username != "" || s.password != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("ClickHouse query failed with status %d", resp.StatusCode)
	}
	return nil
}

func jsonEachRow(rows interface{}) (string, error) {
	var values []interface{}
	switch v := rows.(type) {
	case []clickHouseUsageLogRow:
		values = make([]interface{}, 0, len(v))
		for i := range v {
			values = append(values, v[i])
		}
	case []clickHouseUsageLogPayloadRow:
		values = make([]interface{}, 0, len(v))
		for i := range v {
			values = append(values, v[i])
		}
	case []clickHouseAuditLogRow:
		values = make([]interface{}, 0, len(v))
		for i := range v {
			values = append(values, v[i])
		}
	default:
		return "", fmt.Errorf("unsupported ClickHouse row batch type %T", rows)
	}
	var builder strings.Builder
	for _, row := range values {
		bytes, err := common.Marshal(row)
		if err != nil {
			return "", err
		}
		builder.Write(bytes)
		builder.WriteByte('\n')
	}
	return builder.String(), nil
}

type clickHouseUsageLogRow struct {
	EventID                      string  `json:"event_id"`
	LogID                        int     `json:"log_id"`
	CreatedAt                    string  `json:"created_at"`
	RequestID                    string  `json:"request_id"`
	UpstreamRequestID            string  `json:"upstream_request_id"`
	UserID                       int     `json:"user_id"`
	Username                     string  `json:"username"`
	UserDisplayName              string  `json:"user_display_name"`
	UserRole                     int     `json:"user_role"`
	UserGroup                    string  `json:"user_group"`
	UserEmail                    string  `json:"user_email"`
	CompanyID                    int64   `json:"company_id"`
	CompanyName                  string  `json:"company_name"`
	CompanyCode                  string  `json:"company_code"`
	DepartmentID                 int64   `json:"department_id"`
	DepartmentName               string  `json:"department_name"`
	DepartmentPath               string  `json:"department_path"`
	DepartmentLevel              int     `json:"department_level"`
	DepartmentL1ID               int64   `json:"department_l1_id"`
	DepartmentL1Name             string  `json:"department_l1_name"`
	DepartmentL2ID               int64   `json:"department_l2_id"`
	DepartmentL2Name             string  `json:"department_l2_name"`
	DepartmentL3ID               int64   `json:"department_l3_id"`
	DepartmentL3Name             string  `json:"department_l3_name"`
	DepartmentL4ID               int64   `json:"department_l4_id"`
	DepartmentL4Name             string  `json:"department_l4_name"`
	ApplicationID                int     `json:"application_id"`
	ApplicationKey               string  `json:"application_key"`
	ApplicationName              string  `json:"application_name"`
	TokenID                      int     `json:"token_id"`
	TokenName                    string  `json:"token_name"`
	TokenGroup                   string  `json:"token_group"`
	TokenQueuePriority           int     `json:"token_queue_priority"`
	TokenQueueTimeoutSeconds     int     `json:"token_queue_timeout_seconds"`
	QueueRequired                int     `json:"queue_required"`
	QueueModelName               string  `json:"queue_model_name"`
	QueuePriorityEffective       int     `json:"queue_priority_effective"`
	QueuePriorityToken           int     `json:"queue_priority_token"`
	QueuePriorityCompany         int     `json:"queue_priority_company"`
	QueueTimeoutEffectiveSeconds int     `json:"queue_timeout_effective_seconds"`
	QueuePositionInitial         int     `json:"queue_position_initial"`
	QueueWaitMs                  int64   `json:"queue_wait_ms"`
	QueueResult                  string  `json:"queue_result"`
	QueueEstimatedPromptTokens   int     `json:"queue_estimated_prompt_tokens"`
	QueueMatchedLongContextTier  int     `json:"queue_matched_long_context_tier"`
	ModelName                    string  `json:"model_name"`
	UpstreamModelName            string  `json:"upstream_model_name"`
	ChannelID                    int     `json:"channel_id"`
	ChannelName                  string  `json:"channel_name"`
	ChannelType                  int     `json:"channel_type"`
	IsMultiKey                   int     `json:"is_multi_key"`
	MultiKeyIndex                int     `json:"multi_key_index"`
	UsingGroup                   string  `json:"using_group"`
	Quota                        int     `json:"quota"`
	PromptTokens                 int     `json:"prompt_tokens"`
	CompletionTokens             int     `json:"completion_tokens"`
	TotalTokens                  int     `json:"total_tokens"`
	BillingSource                string  `json:"billing_source"`
	ModelRatio                   float64 `json:"model_ratio"`
	GroupRatio                   float64 `json:"group_ratio"`
	CompletionRatio              float64 `json:"completion_ratio"`
	CacheTokens                  int     `json:"cache_tokens"`
	CacheRatio                   float64 `json:"cache_ratio"`
	RequestMethod                string  `json:"request_method"`
	RequestPath                  string  `json:"request_path"`
	ClientIP                     string  `json:"client_ip"`
	UserAgent                    string  `json:"user_agent"`
	IsStream                     int     `json:"is_stream"`
	OtherJSON                    string  `json:"other_json"`
}

type clickHouseUsageLogPayloadRow struct {
	EventID                     string `json:"event_id"`
	LogID                       int    `json:"log_id"`
	RequestID                   string `json:"request_id"`
	CreatedAt                   string `json:"created_at"`
	ClientRequestHeadersJSON    string `json:"client_request_headers_json"`
	ClientRequestBody           string `json:"client_request_body"`
	UpstreamRequestHeadersJSON  string `json:"upstream_request_headers_json"`
	UpstreamRequestBody         string `json:"upstream_request_body"`
	UpstreamResponseHeadersJSON string `json:"upstream_response_headers_json"`
	UpstreamResponseBody        string `json:"upstream_response_body"`
	ClientResponseHeadersJSON   string `json:"client_response_headers_json"`
	ClientResponseBody          string `json:"client_response_body"`
	ErrorBody                   string `json:"error_body"`
	PayloadSizeBytes            int    `json:"payload_size_bytes"`
	Truncated                   int    `json:"truncated"`
	CaptureMode                 string `json:"capture_mode"`
}

type clickHouseAuditLogRow struct {
	EventID           string `json:"event_id"`
	CreatedAt         string `json:"created_at"`
	ActorUserID       int    `json:"actor_user_id"`
	ActorUsername     string `json:"actor_username"`
	ActorRole         int    `json:"actor_role"`
	ActorCompanyID    int64  `json:"actor_company_id"`
	ActorDepartmentID int64  `json:"actor_department_id"`
	TargetType        string `json:"target_type"`
	TargetID          string `json:"target_id"`
	TargetName        string `json:"target_name"`
	Action            string `json:"action"`
	Result            string `json:"result"`
	Summary           string `json:"summary"`
	DiffJSON          string `json:"diff_json"`
	RequestID         string `json:"request_id"`
	RequestMethod     string `json:"request_method"`
	RequestPath       string `json:"request_path"`
	ClientIP          string `json:"client_ip"`
	UserAgent         string `json:"user_agent"`
	ApplicationID     int    `json:"application_id"`
	ApplicationKey    string `json:"application_key"`
	ApplicationName   string `json:"application_name"`
}

func buildClickHouseUsageLogRow(event *model.UsageLogEvent) clickHouseUsageLogRow {
	ctx := event.RequestContext
	l1ID, l1Name := departmentLevel(ctx.Organization.DepartmentHierarchy, 1)
	l2ID, l2Name := departmentLevel(ctx.Organization.DepartmentHierarchy, 2)
	l3ID, l3Name := departmentLevel(ctx.Organization.DepartmentHierarchy, 3)
	l4ID, l4Name := departmentLevel(ctx.Organization.DepartmentHierarchy, 4)
	otherJSON := ""
	if event.Other != nil {
		if bytes, err := common.Marshal(event.Other); err == nil {
			otherJSON = string(bytes)
		}
	}
	return clickHouseUsageLogRow{
		EventID:                      event.EventID,
		LogID:                        event.LogID,
		CreatedAt:                    clickHouseTime(event.CreatedAt),
		RequestID:                    event.RequestID,
		UpstreamRequestID:            event.UpstreamRequestID,
		UserID:                       event.UserID,
		Username:                     event.Username,
		UserDisplayName:              ctx.User.DisplayName,
		UserRole:                     ctx.User.Role,
		UserGroup:                    ctx.User.Group,
		UserEmail:                    ctx.User.Email,
		CompanyID:                    ctx.Organization.CompanyId,
		CompanyName:                  ctx.Organization.CompanyName,
		CompanyCode:                  ctx.Organization.CompanyCode,
		DepartmentID:                 ctx.Organization.DepartmentId,
		DepartmentName:               ctx.Organization.DepartmentName,
		DepartmentPath:               ctx.Organization.DepartmentPath,
		DepartmentLevel:              ctx.Organization.DepartmentLevel,
		DepartmentL1ID:               l1ID,
		DepartmentL1Name:             l1Name,
		DepartmentL2ID:               l2ID,
		DepartmentL2Name:             l2Name,
		DepartmentL3ID:               l3ID,
		DepartmentL3Name:             l3Name,
		DepartmentL4ID:               l4ID,
		DepartmentL4Name:             l4Name,
		ApplicationID:                ctx.Application.Id,
		ApplicationKey:               ctx.Application.Key,
		ApplicationName:              ctx.Application.Name,
		TokenID:                      event.TokenID,
		TokenName:                    event.TokenName,
		TokenGroup:                   ctx.Token.Group,
		TokenQueuePriority:           ctx.Token.QueuePriority,
		TokenQueueTimeoutSeconds:     ctx.Token.QueueTimeoutSeconds,
		QueueRequired:                boolInt(ctx.Queue.Required),
		QueueModelName:               ctx.Queue.ModelName,
		QueuePriorityEffective:       ctx.Queue.PriorityEffective,
		QueuePriorityToken:           ctx.Queue.PriorityToken,
		QueuePriorityCompany:         ctx.Queue.PriorityCompany,
		QueueTimeoutEffectiveSeconds: ctx.Queue.TimeoutEffectiveSeconds,
		QueuePositionInitial:         ctx.Queue.PositionInitial,
		QueueWaitMs:                  ctx.Queue.WaitMs,
		QueueResult:                  ctx.Queue.Result,
		QueueEstimatedPromptTokens:   ctx.Queue.EstimatedPromptTokens,
		QueueMatchedLongContextTier:  ctx.Queue.MatchedLongContextTier,
		ModelName:                    event.ModelName,
		UpstreamModelName:            ctx.Routing.UpstreamModelName,
		ChannelID:                    event.ChannelID,
		ChannelName:                  ctx.Routing.ChannelName,
		ChannelType:                  ctx.Routing.ChannelType,
		IsMultiKey:                   boolInt(ctx.Routing.IsMultiKey),
		MultiKeyIndex:                ctx.Routing.MultiKeyIndex,
		UsingGroup:                   event.Group,
		Quota:                        event.Quota,
		PromptTokens:                 event.PromptTokens,
		CompletionTokens:             event.CompletionTokens,
		TotalTokens:                  event.PromptTokens + event.CompletionTokens,
		BillingSource:                ctx.Billing.BillingSource,
		ModelRatio:                   ctx.Billing.ModelRatio,
		GroupRatio:                   ctx.Billing.GroupRatio,
		CompletionRatio:              ctx.Billing.CompletionRatio,
		CacheTokens:                  ctx.Billing.CacheTokens,
		CacheRatio:                   ctx.Billing.CacheRatio,
		RequestMethod:                ctx.Request.Method,
		RequestPath:                  ctx.Request.Path,
		ClientIP:                     event.IP,
		UserAgent:                    ctx.Request.UserAgent,
		IsStream:                     boolInt(event.IsStream),
		OtherJSON:                    otherJSON,
	}
}

func buildClickHouseUsageLogPayloadRow(event *model.UsageLogEvent) clickHouseUsageLogPayloadRow {
	payload := event.Payload
	if payload == nil {
		payload = &model.UsageLogPayload{}
	}
	return clickHouseUsageLogPayloadRow{
		EventID:                     event.EventID,
		LogID:                       event.LogID,
		RequestID:                   event.RequestID,
		CreatedAt:                   clickHouseTime(event.CreatedAt),
		ClientRequestHeadersJSON:    payload.ClientRequestHeadersJson,
		ClientRequestBody:           payload.ClientRequestBody,
		UpstreamRequestHeadersJSON:  payload.UpstreamRequestHeadersJson,
		UpstreamRequestBody:         payload.UpstreamRequestBody,
		UpstreamResponseHeadersJSON: payload.UpstreamResponseHeadersJson,
		UpstreamResponseBody:        payload.UpstreamResponseBody,
		ClientResponseHeadersJSON:   payload.ClientResponseHeadersJson,
		ClientResponseBody:          payload.ClientResponseBody,
		ErrorBody:                   payload.ErrorBody,
		PayloadSizeBytes:            payload.PayloadSizeBytes,
		Truncated:                   boolInt(payload.Truncated),
		CaptureMode:                 payload.CaptureMode,
	}
}

func buildClickHouseAuditLogRow(event *model.AuditLogEvent) clickHouseAuditLogRow {
	return clickHouseAuditLogRow{
		EventID:           event.EventID,
		CreatedAt:         clickHouseTime(event.CreatedAt),
		ActorUserID:       event.ActorUserID,
		ActorUsername:     event.ActorUsername,
		ActorRole:         event.ActorRole,
		ActorCompanyID:    event.ActorCompanyID,
		ActorDepartmentID: event.ActorDepartmentID,
		TargetType:        event.TargetType,
		TargetID:          event.TargetID,
		TargetName:        event.TargetName,
		Action:            event.Action,
		Result:            event.Result,
		Summary:           event.Summary,
		DiffJSON:          event.DiffJSON,
		RequestID:         event.RequestID,
		RequestMethod:     event.RequestMethod,
		RequestPath:       event.RequestPath,
		ClientIP:          event.ClientIP,
		UserAgent:         event.UserAgent,
		ApplicationID:     event.ApplicationID,
		ApplicationKey:    event.ApplicationKey,
		ApplicationName:   event.ApplicationName,
	}
}

func departmentLevel(items []model.DepartmentHierarchyItem, level int) (int64, string) {
	for _, item := range items {
		if item.Level == level {
			return item.Id, item.Name
		}
	}
	if len(items) >= level {
		item := items[level-1]
		return item.Id, item.Name
	}
	return 0, ""
}

func clickHouseTime(timestamp int64) string {
	if timestamp <= 0 {
		timestamp = common.GetTimestamp()
	}
	return time.Unix(timestamp, 0).UTC().Format("2006-01-02 15:04:05")
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *clickHouseSink) usageLogsDDL() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS usage_logs (
event_id String,
log_id Int64,
created_at DateTime,
request_id String,
upstream_request_id String,
user_id Int64,
username String,
user_display_name String,
user_role Int32,
user_group String,
user_email String,
company_id Int64,
company_name String,
company_code String,
department_id Int64,
department_name String,
department_path String,
department_level Int32,
department_l1_id Int64,
department_l1_name String,
department_l2_id Int64,
department_l2_name String,
department_l3_id Int64,
department_l3_name String,
department_l4_id Int64,
department_l4_name String,
application_id Int64,
application_key String,
application_name String,
token_id Int64,
token_name String,
token_group String,
token_queue_priority Int32,
token_queue_timeout_seconds Int32,
queue_required UInt8,
queue_model_name String,
queue_priority_effective Int32,
queue_priority_token Int32,
queue_priority_company Int32,
queue_timeout_effective_seconds Int32,
queue_position_initial Int32,
queue_wait_ms Int64,
queue_result String,
queue_estimated_prompt_tokens Int64,
queue_matched_long_context_tier Int64,
model_name String,
upstream_model_name String,
channel_id Int64,
channel_name String,
channel_type Int32,
is_multi_key UInt8,
multi_key_index Int32,
using_group String,
quota Int64,
prompt_tokens Int64,
completion_tokens Int64,
total_tokens Int64,
billing_source String,
model_ratio Float64,
group_ratio Float64,
completion_ratio Float64,
cache_tokens Int64,
cache_ratio Float64,
request_method String,
request_path String,
client_ip String,
user_agent String,
is_stream UInt8,
other_json String
) ENGINE = ReplacingMergeTree
ORDER BY (event_id)
TTL created_at + INTERVAL %d DAY`, s.ttlDays)
}

func (s *clickHouseSink) usageLogPayloadsDDL() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS usage_log_payloads (
event_id String,
log_id Int64,
request_id String,
created_at DateTime,
client_request_headers_json String,
client_request_body String,
upstream_request_headers_json String,
upstream_request_body String,
upstream_response_headers_json String,
upstream_response_body String,
client_response_headers_json String,
client_response_body String,
error_body String,
payload_size_bytes Int64,
truncated UInt8,
capture_mode String
) ENGINE = ReplacingMergeTree
ORDER BY (event_id)
TTL created_at + INTERVAL %d DAY`, s.ttlDays)
}

func (s *clickHouseSink) auditLogsDDL() string {
	return `CREATE TABLE IF NOT EXISTS audit_logs (
event_id String,
created_at DateTime,
actor_user_id Int64,
actor_username String,
actor_role Int32,
actor_company_id Int64,
actor_department_id Int64,
target_type String,
target_id String,
target_name String,
action String,
result String,
summary String,
diff_json String,
request_id String,
request_method String,
request_path String,
client_ip String,
user_agent String,
application_id Int64,
application_key String,
application_name String
) ENGINE = ReplacingMergeTree
ORDER BY (event_id)`
}
