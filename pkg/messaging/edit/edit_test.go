package edit

import (
	"encoding/json"
	"testing"
)

func TestBuildEditContext(t *testing.T) {
	ctx, err := BuildEditContext("mid.$12345", "Updated message content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == "" {
		t.Fatal("expected non-empty context")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(ctx), &parsed); err != nil {
		t.Fatalf("failed to unmarshal outer wrapper: %v", err)
	}

	if parsed["label"] != "21" {
		t.Errorf("expected label '21', got %v", parsed["label"])
	}

	payloadStr, ok := parsed["payload"].(string)
	if !ok || payloadStr == "" {
		t.Fatalf("missing payload string")
	}

	var inner map[string]any
	if err := json.Unmarshal([]byte(payloadStr), &inner); err != nil {
		t.Fatalf("failed to unmarshal inner payload: %v", err)
	}

	tasks, ok := inner["tasks"].([]any)
	if !ok || len(tasks) == 0 {
		t.Fatalf("expected non-empty tasks list")
	}
}
