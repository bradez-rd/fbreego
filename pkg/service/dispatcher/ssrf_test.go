package dispatcher

import (
	"context"
	"testing"
)

func TestIsSSRFRisk(t *testing.T) {
	tests := []struct {
		url      string
		wantRisk bool
	}{
		{"http://127.0.0.1:8080/webhook", true},
		{"http://localhost:5000/hook", true},
		{"http://10.0.0.5/api", true},
		{"http://172.16.1.10:9000/", true},
		{"http://192.168.1.50/alert", true},
		{"http://169.254.169.254/latest/meta-data/", true}, // AWS metadata
		{"http://[::1]:8080/", true},                       // IPv6 loopback
		{"http://0.0.0.0:8000/", true},
		{"invalid://url", true},
		{"file:///etc/passwd", true},
		{"ftp://example.com/file", true},
		{"https://example.com:8443/hook", true},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := IsSSRFRisk(tt.url)
			if got != tt.wantRisk {
				t.Errorf("IsSSRFRisk(%q) = %v, want %v", tt.url, got, tt.wantRisk)
			}
		})
	}
}

func TestSafeDialRejectsPrivateAddress(t *testing.T) {
	if _, err := safeDialContext(context.Background(), "tcp", "127.0.0.1:8080"); err == nil {
		t.Fatal("expected private dial target to be rejected")
	}
}
