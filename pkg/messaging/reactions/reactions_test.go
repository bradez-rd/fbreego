package reactions

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

func TestReactMessage(t *testing.T) {
	mockResponse := `for (;;);{"data":{"message_reaction":{"status":"SUCCESS"}}}`

	client := &http.Client{
		Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
			if req.Method != "POST" {
				t.Errorf("expected POST, got %s", req.Method)
			}
			body, _ := io.ReadAll(req.Body)
			if !bytes.Contains(body, []byte("ADD_REACTION")) {
				t.Errorf("missing ADD_REACTION in body: %s", string(body))
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(mockResponse)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	fb := &types.DataFB{
		FacebookID:     "10001",
		FbDtsg:         "dtsg_token",
		Jazoest:        "2345",
		ClientRevision: "100",
		CookieFacebook: "c_user=10001; xs=abc;",
	}

	res, err := ReactMessage(context.Background(), client, fb, "mid.$123", "❤️", "add")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil response")
	}
	data, ok := res["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing data: %v", res)
	}
	reaction, ok := data["message_reaction"].(map[string]any)
	if !ok || reaction["status"] != "SUCCESS" {
		t.Fatalf("unexpected reaction response: %v", reaction)
	}
}
