package thread

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
