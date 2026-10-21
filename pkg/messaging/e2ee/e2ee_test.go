package e2ee

import (
	"context"
	"errors"
	"testing"
)

func TestProtocolManagerReportsUnavailable(t *testing.T) {
	mgr := NewProtocolManager()
	defer mgr.Close()
	if mgr.Available() {
		t.Fatal("protocol must not advertise availability without a real backend")
	}
	res, err := mgr.Actions().EditMessage(context.Background(), "mid.$123", "edited")
	if res != nil || !errors.Is(err, ErrProtocolUnavailable) {
		t.Fatalf("expected ErrProtocolUnavailable and nil result, got result=%v err=%v", res, err)
	}
}

func TestProtocolManagerHonorsCancelledContext(t *testing.T) {
	mgr := NewProtocolManager()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := mgr.Call(ctx, "editMessage", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
