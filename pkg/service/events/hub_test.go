package events

import (
	"testing"
)

type recordingDispatcher struct {
	eventName string
	payload   map[string]any
}

func (d *recordingDispatcher) Enqueue(eventName string, payload map[string]any) bool {
	d.eventName = eventName
	d.payload = payload
	return true
}

func TestRealtimeHub(t *testing.T) {
	hub := NewRealtimeHub()

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}

	// Register fake subscribers (ws can be nil since we test channel broadcast)
	sub1 := &EventSubscriber{
		sendChan:  make(chan []byte, 10),
		sessionID: "sess_1",
	}
	sub2 := &EventSubscriber{
		sendChan:  make(chan []byte, 10),
		sessionID: "", // global listener
	}

	hub.mu.Lock()
	hub.subscribers[sub1] = true
	hub.subscribers[sub2] = true
	hub.mu.Unlock()

	if hub.ClientCount() != 2 {
		t.Errorf("expected 2 clients, got %d", hub.ClientCount())
	}

	// Broadcast targeted to sess_1
	hub.EmitSessionStatus("sess_1", "CONNECTED", "")

	select {
	case msg := <-sub1.sendChan:
		if len(msg) == 0 {
			t.Errorf("empty message for sub1")
		}
	default:
		t.Errorf("expected message in sub1 channel")
	}

	select {
	case msg := <-sub2.sendChan:
		if len(msg) == 0 {
			t.Errorf("empty message for global sub2")
		}
	default:
		t.Errorf("expected message in sub2 channel")
	}

	// Unregister
	hub.Unregister(sub1)
	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client after unregistering sub1, got %d", hub.ClientCount())
	}
}

func TestRealtimeHubPublishesToDispatcher(t *testing.T) {
	hub := NewRealtimeHub()
	dispatcher := &recordingDispatcher{}
	hub.SetDispatcher(dispatcher)
	hub.EmitSessionStatus("sess_1", "active", "")
	if dispatcher.eventName != "session_status" {
		t.Fatalf("expected session_status dispatch, got %q", dispatcher.eventName)
	}
	if dispatcher.payload["sessionId"] != "sess_1" {
		t.Fatalf("unexpected payload: %v", dispatcher.payload)
	}
}
