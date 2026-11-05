package webhook

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWebhookOutboxPersistsEncryptedJobsAndReloadsThem(t *testing.T) {
	t.Setenv("FBREEGO_SESSION_SECRET", "test-only-session-secret-32-characters-minimum")
	dir := t.TempDir()
	store := NewWebhookOutboxStore(dir)
	job, err := store.Add("message.received", map[string]any{
		"sessionId": "session-1",
		"messageId": "sensitive-message-id",
	})
	if err != nil {
		t.Fatalf("add outbox job: %v", err)
	}
	if store.Count() != 1 {
		t.Fatalf("expected one persisted job, got %d", store.Count())
	}

	raw, err := os.ReadFile(filepath.Join(dir, webhookOutboxFilename))
	if err != nil {
		t.Fatalf("read encrypted outbox: %v", err)
	}
	if bytes.Contains(raw, []byte("sensitive-message-id")) || bytes.Contains(raw, []byte("message.received")) {
		t.Fatal("outbox must not persist event data in plaintext")
	}

	reloaded := NewWebhookOutboxStore(dir)
	due := reloaded.Due(time.Now().Add(time.Second))
	if len(due) != 1 || due[0].ID != job.ID || due[0].EventName != "message.received" {
		t.Fatalf("unexpected restored jobs: %#v", due)
	}
	if err := reloaded.MarkFailed(job.ID, os.ErrDeadlineExceeded, time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	if len(reloaded.Due(time.Now())) != 0 {
		t.Fatal("job must respect its next retry time")
	}
	if err := reloaded.Delete(job.ID); err != nil {
		t.Fatalf("delete job: %v", err)
	}
	if reloaded.Count() != 0 {
		t.Fatalf("expected empty outbox, got %d", reloaded.Count())
	}
}
