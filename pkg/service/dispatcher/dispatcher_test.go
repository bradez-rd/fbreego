package dispatcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bradez-rd/fbreego/pkg/service/webhook"
)

func TestDispatcherDeliversLegacyAliasAsCanonicalEventToAllowedSocialManagerOrigin(t *testing.T) {
	t.Setenv("FBREEGO_SESSION_SECRET", "test-only-session-secret-32-characters-minimum")

	received := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode webhook: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	t.Setenv("FBREEGO_WEBHOOK_ALLOWED_TARGETS", server.URL)

	store := webhook.NewWebhookConfigStore(t.TempDir())
	secret := "webhook-signing-secret"
	if _, err := store.Update(true, server.URL+"/api/social/webhooks/providers/provider-1", []string{"message.received"}, &secret, false); err != nil {
		t.Fatalf("configure webhook: %v", err)
	}

	dispatcher := NewDispatcher(store)
	if err := dispatcher.deliverWithRetry(context.Background(), "message_received", map[string]any{
		"sessionId": "session-1",
		"messageId": "message-1",
	}); err != nil {
		t.Fatalf("deliver webhook: %v", err)
	}

	body := <-received
	if body["event"] != "message.received" {
		t.Fatalf("expected canonical event name, got %v", body["event"])
	}
	payload, ok := body["payload"].(map[string]any)
	if !ok || payload["messageId"] != "message-1" {
		t.Fatalf("unexpected webhook payload: %#v", body["payload"])
	}
	if stats := dispatcher.Stats(); stats["totalDelivered"] != int64(1) || stats["totalFailed"] != int64(0) {
		t.Fatalf("unexpected dispatcher stats: %#v", stats)
	}
}

func TestDispatcherRecoversPersistedWebhookAfterRestart(t *testing.T) {
	t.Setenv("FBREEGO_SESSION_SECRET", "test-only-session-secret-32-characters-minimum")

	received := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv("FBREEGO_WEBHOOK_ALLOWED_TARGETS", server.URL)

	stateDir := t.TempDir()
	store := webhook.NewWebhookConfigStore(stateDir)
	secret := "webhook-signing-secret"
	if _, err := store.Update(true, server.URL+"/api/social/webhooks/providers/provider-1", []string{"message.received"}, &secret, false); err != nil {
		t.Fatalf("configure webhook: %v", err)
	}

	beforeRestart := NewDispatcher(store)
	if !beforeRestart.Enqueue("message_received", map[string]any{"messageId": "message-1"}) {
		t.Fatal("expected event to be durably accepted")
	}
	if pending := beforeRestart.Stats()["persistentPending"]; pending != 1 {
		t.Fatalf("expected one durable pending job, got %v", pending)
	}

	afterRestart := NewDispatcher(store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	afterRestart.Start(ctx)
	defer afterRestart.Stop()

	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for recovered webhook delivery")
	}

	deadline := time.Now().Add(2 * time.Second)
	for afterRestart.Stats()["persistentPending"] != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if pending := afterRestart.Stats()["persistentPending"]; pending != 0 {
		t.Fatalf("expected recovered job to be acknowledged, got %v", pending)
	}
}
