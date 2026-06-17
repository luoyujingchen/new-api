package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

func TestKafkaNativeSinkPublishesToBroker(t *testing.T) {
	rawBrokers := strings.TrimSpace(os.Getenv("NEWAPI_LOG_OUTBOX_INTEGRATION_KAFKA_BROKERS"))
	if rawBrokers == "" {
		t.Skip("set NEWAPI_LOG_OUTBOX_INTEGRATION_KAFKA_BROKERS to run Kafka integration test")
	}
	brokers := splitKafkaBrokers(rawBrokers)
	require.NotEmpty(t, brokers)

	topic := fmt.Sprintf("newapi-log-outbox-it-%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	require.NoError(t, err)
	require.NoError(t, conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}))
	t.Cleanup(func() {
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		_ = conn.DeleteTopics(topic)
		_ = conn.Close()
	})

	event := model.CanonicalLogEvent{
		Version:   1,
		EventID:   "usage-kafka-it",
		EventType: model.LogOutboxEventTypeUsage,
		Usage: &model.UsageLogEvent{
			EventID:   "usage-kafka-it",
			LogID:     1001,
			UserID:    42,
			Username:  "alice",
			CreatedAt: common.GetTimestamp(),
			Payload: &model.UsageLogPayload{
				ClientRequestBody: "sensitive prompt",
			},
		},
	}
	rawEvent, err := common.Marshal(event)
	require.NoError(t, err)

	sink := &kafkaNativeSink{
		brokers:    brokers,
		usageTopic: topic,
		auditTopic: topic + "-audit",
	}
	require.NoError(t, sink.Publish(ctx, []model.LogOutbox{{
		EventID:     event.EventID,
		EventType:   model.LogOutboxEventTypeUsage,
		PayloadJSON: model.LogOutboxPayloadJSON(rawEvent),
	}}))

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   brokers,
		Topic:     topic,
		Partition: 0,
		MinBytes:  1,
		MaxBytes:  1024 * 1024,
		MaxWait:   time.Second,
	})
	defer reader.Close()

	message, err := reader.ReadMessage(ctx)
	require.NoError(t, err)
	require.Equal(t, event.EventID, string(message.Key))

	var published model.CanonicalLogEvent
	require.NoError(t, common.Unmarshal(message.Value, &published))
	require.NotNil(t, published.Usage)
	require.Nil(t, published.Usage.Payload)
	require.NotContains(t, string(message.Value), "sensitive prompt")
}

func TestClickHouseSinkPublishesUsagePayload(t *testing.T) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("NEWAPI_LOG_OUTBOX_INTEGRATION_CLICKHOUSE_URL")), "/")
	if baseURL == "" {
		t.Skip("set NEWAPI_LOG_OUTBOX_INTEGRATION_CLICKHOUSE_URL to run ClickHouse integration test")
	}
	database := os.Getenv("NEWAPI_LOG_OUTBOX_INTEGRATION_CLICKHOUSE_DATABASE")
	username := os.Getenv("NEWAPI_LOG_OUTBOX_INTEGRATION_CLICKHOUSE_USERNAME")
	password := os.Getenv("NEWAPI_LOG_OUTBOX_INTEGRATION_CLICKHOUSE_PASSWORD")

	sink := &clickHouseSink{
		baseURL:  baseURL,
		database: database,
		username: username,
		password: password,
		ttlDays:  1,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	require.NoError(t, sink.EnsureSchema(ctx))

	eventID := fmt.Sprintf("usage-clickhouse-it-%d", time.Now().UnixNano())
	event := model.CanonicalLogEvent{
		Version:   1,
		EventID:   eventID,
		EventType: model.LogOutboxEventTypeUsage,
		Usage: &model.UsageLogEvent{
			EventID:          eventID,
			LogID:            2001,
			UserID:           42,
			Username:         "alice",
			CreatedAt:        common.GetTimestamp(),
			RequestID:        "req-clickhouse-it",
			ModelName:        "gpt-test",
			PromptTokens:     3,
			CompletionTokens: 4,
			Payload: &model.UsageLogPayload{
				ClientRequestBody: "hello clickhouse payload",
				CaptureMode:       "text:65536",
			},
		},
	}
	rawEvent, err := common.Marshal(event)
	require.NoError(t, err)
	require.NoError(t, sink.Publish(ctx, []model.LogOutbox{{
		EventID:     event.EventID,
		EventType:   model.LogOutboxEventTypeUsage,
		PayloadJSON: model.LogOutboxPayloadJSON(rawEvent),
	}}))

	payloadBody, err := clickHouseIntegrationQuery(ctx, sink, fmt.Sprintf(
		"SELECT client_request_body FROM usage_log_payloads WHERE event_id = '%s' FORMAT TabSeparatedRaw",
		eventID,
	))
	require.NoError(t, err)
	require.Equal(t, "hello clickhouse payload", strings.TrimSpace(payloadBody))

	usageCount, err := clickHouseIntegrationQuery(ctx, sink, fmt.Sprintf(
		"SELECT count() FROM usage_logs WHERE event_id = '%s'",
		eventID,
	))
	require.NoError(t, err)
	require.Equal(t, "1", strings.TrimSpace(usageCount))
}

func splitKafkaBrokers(raw string) []string {
	parts := strings.Split(raw, ",")
	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			brokers = append(brokers, part)
		}
	}
	return brokers
}

func clickHouseIntegrationQuery(ctx context.Context, sink *clickHouseSink, query string) (string, error) {
	endpoint, err := url.Parse(sink.baseURL + "/")
	if err != nil {
		return "", err
	}
	values := endpoint.Query()
	values.Set("query", query)
	if sink.database != "" {
		values.Set("database", sink.database)
	}
	endpoint.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return "", err
	}
	if sink.username != "" || sink.password != "" {
		req.SetBasicAuth(sink.username, sink.password)
	}
	resp, err := sink.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", readErr
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("ClickHouse query failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}
