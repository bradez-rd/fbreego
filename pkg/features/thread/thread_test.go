package thread

import (
	"bytes"
	"context"
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

func TestFetchAllThreadsUsesConfiguredContract(t *testing.T) {
	t.Setenv("FBCHAT_URL_GRAPHQLBATCH", "https://contracts.example/graphqlbatch/")
	t.Setenv("FBCHAT_DOCID_THREADLISTQUERY", "thread-list-override")

	client := &http.Client{Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://contracts.example/graphqlbatch/" {
			t.Fatalf("unexpected endpoint: %s", req.URL)
		}
		if err := req.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		query := req.Form.Get("queries")
		if !strings.Contains(query, `"doc_id":"thread-list-override"`) {
			t.Fatalf("configured doc id missing from query: %s", query)
		}
		if !strings.Contains(query, `"limit":50`) || !strings.Contains(query, `"includeSeqID":true`) {
			t.Fatalf("merged query parameters missing: %s", query)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"o0":{"data":{}}}`)),
			Header:     make(http.Header),
		}, nil
	})}

	_, err := FetchAllThreads(context.Background(), client, &types.DataFB{FacebookID: "10001"})
	if err != nil {
		t.Fatalf("FetchAllThreads failed: %v", err)
	}
}

func TestFetchAllThreadsClassifiesHTTPError(t *testing.T) {
	client := &http.Client{Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(strings.NewReader(`{"error":"rate limited"}`)),
			Header:     http.Header{"Retry-After": []string{"30"}},
		}, nil
	})}

	_, err := FetchAllThreads(context.Background(), client, &types.DataFB{})
	if !errors.Is(err, httputil.ErrRateLimited) {
		t.Fatalf("expected rate limit classification, got %v", err)
	}
}

func TestCustomization(t *testing.T) {
	mockResponse := `for (;;);{"payload":{"success":true}}`

	client := &http.Client{
		Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(mockResponse)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	fb := &types.DataFB{FacebookID: "10001"}

	// ChangeThreadName
	resName, err := ChangeThreadName(context.Background(), client, fb, "group_123", "New Name")
	if err != nil || resName == nil {
		t.Fatalf("ChangeThreadName failed: %v", err)
	}

	// ChangeEmoji
	resEmoji, err := ChangeEmoji(context.Background(), client, fb, "group_123", "🚀")
	if err != nil || resEmoji == nil {
		t.Fatalf("ChangeEmoji failed: %v", err)
	}

	// ChangeNickname
	resNick, err := ChangeNickname(context.Background(), client, fb, "group_123", "user_456", "Johnny")
	if err != nil || resNick == nil {
		t.Fatalf("ChangeNickname failed: %v", err)
	}
}

func TestManageAdmin(t *testing.T) {
	mockResponse := `for (;;);{"payload":{"success":true}}`

	client := &http.Client{
		Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(mockResponse)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	fb := &types.DataFB{FacebookID: "10001"}

	// Add Admin
	resAdd, err := ManageAdmin(context.Background(), client, fb, "group_123", "user_456", true)
	if err != nil || resAdd == nil {
		t.Fatalf("ManageAdmin add failed: %v", err)
	}

	// Remove Admin
	resRemove, err := ManageAdmin(context.Background(), client, fb, "group_123", "user_456", false)
	if err != nil || resRemove == nil {
		t.Fatalf("ManageAdmin remove failed: %v", err)
	}
}
