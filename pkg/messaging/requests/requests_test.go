package requests

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

func TestGetPendingRequests(t *testing.T) {
	mockResponse := `for (;;);{"o0":{"data":{"viewer":{"message_threads":{"nodes":[]}}}}}`

	client := &http.Client{
		Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			if !bytes.Contains(body, []byte("PENDING")) {
				t.Errorf("missing PENDING query param: %s", string(body))
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(mockResponse)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	fb := &types.DataFB{FacebookID: "10001"}
	res, err := GetPendingRequests(context.Background(), client, fb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil response")
	}
}
