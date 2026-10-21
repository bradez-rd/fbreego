package listener

import (
	"context"
	"errors"
	"testing"
)

func TestMQTTListenerDoesNotReportFalseSuccess(t *testing.T) {
	listener := NewMQTTListener(nil)
	if err := listener.Start(context.Background()); !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("expected ErrTransportUnavailable, got %v", err)
	}
}
