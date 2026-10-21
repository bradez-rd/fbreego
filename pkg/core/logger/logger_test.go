package logger

import (
	"testing"
)

func TestRedactSensitive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Redact cookie c_user",
			input:    "Cookie: c_user=100012345; xs=abcdef123;",
			expected: "Cookie: c_user=<redacted>; xs=<redacted>;",
		},
		{
			name:     "Redact JSON secret fields",
			input:    `{"cookie": "c_user=123", "password": "supersecretpassword", "access_token": "EAA..."}`,
			expected: `{"cookie": "<redacted>", "password": "<redacted>", "access_token": "<redacted>"}`,
		},
		{
			name:     "Normal text remains unchanged",
			input:    "Listening on :8000 successfully",
			expected: "Listening on :8000 successfully",
		},
		{
			name:     "Redact cookie collection value",
			input:    `{"cookies":[{"name":"xs","value":"session-secret"},{"name":"locale","value":"es_ES"}]}`,
			expected: `{"cookies":[{"name":"xs","value":"<redacted>"},{"name":"locale","value":"es_ES"}]}`,
		},
		{
			name:     "Redact interpolated-style plain secret",
			input:    "request failed password: my_pass",
			expected: "request failed password: <redacted>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactSensitive(tc.input)
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestLoggerCalls(t *testing.T) {
	Info("Testing Info log with c_user=12345")
	Error("Testing Error log with password: %s", "my_pass")
	Debug("Testing Debug log")
}
