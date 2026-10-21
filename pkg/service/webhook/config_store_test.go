package webhook

import (
	"os"
	"testing"
)

func TestWebhookConfigStore(t *testing.T) {
	t.Setenv("FBCHAT_SESSION_SECRET", "test-only-session-secret-32-characters-minimum")
	tmpDir, err := os.MkdirTemp("", "fbreego_webhook_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewWebhookConfigStore(tmpDir)

	// Initially empty config
	initial := store.Get()
	if initial.Enabled {
		t.Errorf("expected initial config to be disabled")
	}

	// Update with invalid URL
	secretVal := "secret123"
	_, err = store.Update(true, "not-a-valid-url", []string{"message"}, &secretVal, false)
	if err == nil {
		t.Errorf("expected error for invalid url, got nil")
	}

	// Update with valid URL and valid events
	sec := "secret_key"
	_, err = store.Update(true, "https://webhook.site/test-endpoint", []string{"message", "session_status"}, &sec, false)
	if err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	cfg := store.Get()
	if !cfg.Enabled || cfg.URL != "https://webhook.site/test-endpoint" {
		t.Errorf("unexpected saved config: %+v", cfg)
	}
	if cfg.Secret != "secret_key" {
		t.Errorf("unexpected secret: %s", cfg.Secret)
	}

	// Dotted UI aliases are normalized to the event names emitted by the hub.
	_, err = store.Update(true, "https://webhook.site/test-endpoint", []string{"message.received", "message_received", "session.status"}, nil, false)
	if err != nil {
		t.Fatalf("unexpected alias normalization error: %v", err)
	}
	cfg = store.Get()
	if len(cfg.Events) != 2 || cfg.Events[0] != "message_received" || cfg.Events[1] != "session_status" {
		t.Fatalf("unexpected normalized events: %v", cfg.Events)
	}

	// Verify encryption on disk: new instance should be able to decrypt
	store2 := NewWebhookConfigStore(tmpDir)
	cfg2 := store2.Get()
	if cfg2.URL != cfg.URL || cfg2.Secret != cfg.Secret {
		t.Errorf("restored config mismatch: %+v vs %+v", cfg2, cfg)
	}
}
