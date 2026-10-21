package send

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/bradez-rd/fbreego/pkg/core/types"
)

type mockTripper func(req *http.Request) (*http.Response, error)

func (m mockTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}

func TestSendTextUser(t *testing.T) {
	mockResponse := `for (;;);{"payload":{"actions":[{"message_id":"mid.$123"}]}}`

	client := &http.Client{
		Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
			if req.Method != "POST" {
				t.Errorf("expected POST, got %s", req.Method)
			}
			if req.URL.String() != SendURL {
				t.Errorf("expected URL %s, got %s", SendURL, req.URL.String())
			}

			body, _ := io.ReadAll(req.Body)
			bodyStr := string(body)
			if !bytes.Contains(body, []byte("body=Hello+World")) {
				t.Errorf("body missing expected message text: %s", bodyStr)
			}
			if !bytes.Contains(body, []byte("other_user_fbid=1000999")) {
				t.Errorf("body missing target fbid: %s", bodyStr)
			}

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(mockResponse)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	fb := &types.DataFB{
		FacebookID:     "1000111",
		FbDtsg:         "NAcPA...",
		Jazoest:        "25432",
		ClientRevision: "1000",
		CookieFacebook: "c_user=1000111; xs=abc;",
	}

	res, err := SendText(context.Background(), client, fb, types.TextMessageInput{
		ThreadID:   "1000999",
		ThreadType: "user",
		Text:       "Hello World",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil response map")
	}

	payload, ok := res["payload"].(map[string]any)
	if !ok {
		t.Fatalf("missing payload: %v", res)
	}
	actions, ok := payload["actions"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatalf("missing actions in payload: %v", payload)
	}
}

func TestSendTextGroup(t *testing.T) {
	mockResponse := `for (;;);{"payload":{"actions":[{"message_id":"mid.$456"}]}}`

	client := &http.Client{
		Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			if !bytes.Contains(body, []byte("thread_fbid=987654321")) {
				t.Errorf("missing thread_fbid: %s", string(body))
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(mockResponse)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	fb := &types.DataFB{FacebookID: "1000111"}
	res, err := SendText(context.Background(), client, fb, types.TextMessageInput{
		ThreadID:         "987654321",
		ThreadType:       "group",
		Text:             "Hello Group",
		ReplyToMessageID: "mid.$001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil response")
	}
}
