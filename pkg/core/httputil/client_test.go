package httputil

import (
	"testing"
	"time"
)

func TestNewDefaultTransport(t *testing.T) {
	tr := NewDefaultTransport("")
	if tr == nil {
		t.Fatal("expected transport, got nil")
	}
	if !tr.ForceAttemptHTTP2 {
		t.Errorf("expected ForceAttemptHTTP2 to be true")
	}

	trProxy := NewDefaultTransport("http://127.0.0.1:8080")
	if trProxy.Proxy == nil {
		t.Errorf("expected Proxy to be configured")
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient(5*time.Second, "")
	if client == nil {
		t.Fatal("expected client, got nil")
	}
	if client.Timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", client.Timeout)
	}

	clientDefault := NewClient(0, "")
	if clientDefault.Timeout != 30*time.Second {
		t.Errorf("expected default 30s timeout, got %v", clientDefault.Timeout)
	}
}
