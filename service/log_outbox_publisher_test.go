package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestKafkaEventPayloadStripsUsagePayload(t *testing.T) {
	event := model.CanonicalLogEvent{
		Version:   1,
		EventID:   "usage-1",
		EventType: model.LogOutboxEventTypeUsage,
		Usage: &model.UsageLogEvent{
			EventID: "usage-1",
			LogID:   1,
			Payload: &model.UsageLogPayload{
				ClientRequestBody: "sensitive prompt",
			},
		},
	}
	raw, err := common.Marshal(event)
	require.NoError(t, err)

	payload, err := kafkaEventPayload(model.LogOutbox{
		EventID:     "usage-1",
		EventType:   model.LogOutboxEventTypeUsage,
		PayloadJSON: model.LogOutboxPayloadJSON(raw),
	})
	require.NoError(t, err)

	var published model.CanonicalLogEvent
	require.NoError(t, common.Unmarshal(payload, &published))
	require.NotNil(t, published.Usage)
	require.Nil(t, published.Usage.Payload)
	require.NotContains(t, string(payload), "sensitive prompt")
}

func TestKafkaEventPayloadKeepsAuditEvent(t *testing.T) {
	event := model.CanonicalLogEvent{
		Version:   1,
		EventID:   "audit-1",
		EventType: model.LogOutboxEventTypeAudit,
		Audit: &model.AuditLogEvent{
			EventID: "audit-1",
			Action:  "payload.view",
			Summary: "view payload",
		},
	}
	raw, err := common.Marshal(event)
	require.NoError(t, err)

	payload, err := kafkaEventPayload(model.LogOutbox{
		EventID:     "audit-1",
		EventType:   model.LogOutboxEventTypeAudit,
		PayloadJSON: model.LogOutboxPayloadJSON(raw),
	})
	require.NoError(t, err)

	var published model.CanonicalLogEvent
	require.NoError(t, common.Unmarshal(payload, &published))
	require.NotNil(t, published.Audit)
	require.Equal(t, "payload.view", published.Audit.Action)
}
