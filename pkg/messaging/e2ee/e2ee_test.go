package e2ee

import (
	"context"
	"testing"
)

func TestProtocolManagerAndActions(t *testing.T) {
	mgr := NewProtocolManager()
	defer mgr.Close()

	actions := mgr.Actions()
	if actions == nil {
		t.Fatal("expected non-nil actions facade")
	}

	ctx := context.Background()

	// Test EditMessage
	if res, err := actions.EditMessage(ctx, "mid.$123", "new edited text"); err != nil || res == nil {
		t.Errorf("EditMessage failed: %v", err)
	}

	// Test UnsendMessage
	if res, err := actions.UnsendMessage(ctx, "mid.$123"); err != nil || res == nil {
		t.Errorf("UnsendMessage failed: %v", err)
	}

	// Test EditE2EEMessage
	if res, err := actions.EditE2EEMessage(ctx, "jid_123", "mid.$123", "new text"); err != nil || res == nil {
		t.Errorf("EditE2EEMessage failed: %v", err)
	}

	// Test UnsendE2EEMessage
	if res, err := actions.UnsendE2EEMessage(ctx, "jid_123", "mid.$123"); err != nil || res == nil {
		t.Errorf("UnsendE2EEMessage failed: %v", err)
	}

	// Test SendTypingIndicator
	if res, err := actions.SendTypingIndicator(ctx, 123456789, true, false); err != nil || res == nil {
		t.Errorf("SendTypingIndicator failed: %v", err)
	}

	// Test SendE2EETyping
	if res, err := actions.SendE2EETyping(ctx, "jid_123", true); err != nil || res == nil {
		t.Errorf("SendE2EETyping failed: %v", err)
	}

	// Test MarkRead
	if res, err := actions.MarkRead(ctx, 123456789, 1600000000000); err != nil || res == nil {
		t.Errorf("MarkRead failed: %v", err)
	}

	// Test SendE2EEAudio
	if res, err := actions.SendE2EEAudio(ctx, "jid_123", []byte("audio-bytes"), "audio/ogg", 5); err != nil || res == nil {
		t.Errorf("SendE2EEAudio failed: %v", err)
	}

	// Test SendE2EEImage
	if res, err := actions.SendE2EEImage(ctx, "jid_123", []byte("image-bytes"), "image/jpeg", "A caption", 800, 600); err != nil || res == nil {
		t.Errorf("SendE2EEImage failed: %v", err)
	}
}
