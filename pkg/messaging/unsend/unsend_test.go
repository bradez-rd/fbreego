package unsend

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

func TestUnsendMessage(t *testing.T) {
	mockResponse := `for (;;);{"payload":{"success":true}}`

	client := &http.Client{
		Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
			if req.Method != "POST" {
				t.Errorf("expected POST, got %s", req.Method)
			}
			body, _ := io.ReadAll(req.Body)
			if !bytes.Contains(body, []byte("message_id=mid.%24999")) && !bytes.Contains(body, []byte("message_id=mid.$999")) {
				t.Errorf("missing message_id in body: %s", string(body))
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
		FbDtsg:         "dtsg_sample",
		Jazoest:        "2345",
		ClientRevision: "100",
		CookieFacebook: "c_user=10001; xs=abc;",
	}

	res, err := UnsendMessage(context.Background(), client, fb, "mid.$999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil response")
	}
	payload, ok := res["payload"].(map[string]any)
	if !ok || payload["success"] != true {
		t.Fatalf("unexpected response payload: %v", res)
	}
}
