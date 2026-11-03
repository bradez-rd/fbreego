package listener

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMQTTListenerDoesNotReportFalseSuccess(t *testing.T) {
	listener := NewMQTTListener(nil)
	if err := listener.Start(context.Background()); !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("expected ErrTransportUnavailable, got %v", err)
	}
}

func decodeTestPayload(t *testing.T, raw string) map[string]any {
	t.Helper()
	var payload map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		t.Fatalf("decode test payload: %v", err)
	}
	return payload
}

func TestDecodePayloadEventsClassifiesMetaAIAsDirect(t *testing.T) {
	payload := decodeTestPayload(t, `{
		"deltas": [{
			"messageMetadata": {
				"threadKey": {"threadFbId": "156025504001094"},
				"messageId": "mid.meta-ai.1",
				"actorFbId": "156025504001094",
				"timestamp": "1793500000000",
				"threadType": 201
			},
			"body": ""
		}]
	}`)

	events := decodePayloadEvents(payload)
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d: %#v", len(events), events)
	}
	event := events[0]
	if event.EventType != MessageReceived || event.IsGroup {
		t.Fatalf("expected direct received event, got %#v", event)
	}
	if event.ThreadType != "201" || event.ThreadID != metaAIThreadID {
		t.Fatalf("unexpected Meta AI identity: %#v", event)
	}
}

func TestDecodePayloadEventsMapsBridgeMessageEdit(t *testing.T) {
	payload := decodeTestPayload(t, `{
		"type": "messageEdit",
		"data": {
			"messageID": "mid.meta-ai.1",
			"threadID": "156025504001094",
			"senderID": "156025504001094",
			"newText": "Respuesta completa de Meta AI",
			"timestamp": 1793500000123,
			"threadType": "201"
		}
	}`)

	events := decodePayloadEvents(payload)
	if len(events) != 1 {
		t.Fatalf("expected one edit event, got %d: %#v", len(events), events)
	}
	event := events[0]
	if event.EventType != MessageEdited || event.Body != "Respuesta completa de Meta AI" {
		t.Fatalf("unexpected edit event: %#v", event)
	}
	if event.IsGroup || event.ThreadID != metaAIThreadID || event.MessageID != "mid.meta-ai.1" {
		t.Fatalf("unexpected edit identity: %#v", event)
	}
}

func TestDecodePayloadEventsMapsNestedDeltaMessageEdit(t *testing.T) {
	payload := decodeTestPayload(t, `{
		"deltas": [{
			"deltaMessageEdit": {
				"messageId": "mid.edit.2",
				"threadKey": {"threadFbId": "9001"},
				"senderId": "9001",
				"body": "Texto final",
				"timestampMs": 1793500000456
			}
		}]
	}`)

	events := decodePayloadEvents(payload)
	if len(events) != 1 || events[0].EventType != MessageEdited {
		t.Fatalf("unexpected nested edit events: %#v", events)
	}
	if events[0].IsGroup {
		t.Fatalf("thread equal to sender must be direct: %#v", events[0])
	}
}
