package edit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bradez-rd/fbreego/pkg/core/httputil"
	"github.com/bradez-rd/fbreego/pkg/core/types"
)

type mockTripper func(req *http.Request) (*http.Response, error)

func (m mockTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}

func TestBuildEditContext(t *testing.T) {
	t.Setenv("FBCHAT_DOCID_EDITMESSAGEVERSION", "edit-version-override")
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
	if inner["version_id"] != "edit-version-override" {
		t.Fatalf("expected configured version id, got %v", inner["version_id"])
	}

	tasks, ok := inner["tasks"].([]any)
	if !ok || len(tasks) == 0 {
		t.Fatalf("expected non-empty tasks list")
	}
}

func TestEditMessageUsesConfiguredContract(t *testing.T) {
	t.Setenv("FBCHAT_URL_GRAPHQLBATCH", "https://contracts.example/graphqlbatch/")
	t.Setenv("FBCHAT_DOCID_EDITMESSAGEVERSION", "edit-version-override")

	client := &http.Client{Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://contracts.example/graphqlbatch/" {
			t.Fatalf("unexpected endpoint: %s", req.URL)
		}
		if err := req.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if query := req.Form.Get("queries"); !strings.Contains(query, `"doc_id":"edit-version-override"`) {
			t.Fatalf("configured version id missing from query: %s", query)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`for (;;);{"payload":{"success":true}}`)),
			Header:     make(http.Header),
		}, nil
	})}

	result, err := EditMessage(context.Background(), client, &types.DataFB{}, "mid.$12345", "updated")
	if err != nil {
		t.Fatalf("EditMessage failed: %v", err)
	}
	if result["status"] != "edited" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestEditMessageClassifiesHTTPError(t *testing.T) {
	client := &http.Client{Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(`{"error":"expired"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	_, err := EditMessage(context.Background(), client, &types.DataFB{}, "mid.$12345", "updated")
	if !errors.Is(err, httputil.ErrSessionExpired) {
		t.Fatalf("expected session-expired classification, got %v", err)
	}
}
