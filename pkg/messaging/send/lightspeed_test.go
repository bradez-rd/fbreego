package send

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/bradez-rd/fbreego/pkg/core/types"
)

func TestSendLightSpeedOpenSuccess(t *testing.T) {
	mockResponse := `{"data":{"viewer":{"lightspeed_web_request":{"payload":"[[\"LSReplaceOptimsiticMessage\",123]]"}}}}`

	client := &http.Client{
		Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
			if req.Method != "POST" {
				t.Errorf("expected POST, got %s", req.Method)
			}
			if req.URL.String() != GraphQLURL {
				t.Errorf("expected URL %s, got %s", GraphQLURL, req.URL.String())
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
		CookieFacebook: "c_user=1000111; xs=abc;",
	}

	res, err := SendLightSpeedOpen(context.Background(), client, fb, types.TextMessageInput{
		ThreadID:   "987654321",
		ThreadType: "group",
		Text:       "Hello LightSpeed Group",
	}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["status"] != "delivered" {
		t.Fatalf("expected delivered status, got: %v", res["status"])
	}
	if res["protocol"] != "lightspeed_open" {
		t.Fatalf("expected lightspeed_open protocol, got: %v", res["protocol"])
	}
}

func TestSendLightSpeedOpenCutoverDetection(t *testing.T) {
	mockResponse := `{"data":{"viewer":{"lightspeed_web_request":{"payload":"[[\"LSCutoverHandleInvalidSendToOpen\",{\"thread_id\":100011614426020}]]"}}}}`

	client := &http.Client{
		Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
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
		CookieFacebook: "c_user=1000111; xs=abc;",
	}

	_, err := SendLightSpeedOpen(context.Background(), client, fb, types.TextMessageInput{
		ThreadID:   "100011614426020",
		ThreadType: "user",
		Text:       "Hello 1-to-1",
	}, nil)

	if !errors.Is(err, ErrCutoverToE2EE) {
		t.Fatalf("expected ErrCutoverToE2EE, got: %v", err)
	}
}
