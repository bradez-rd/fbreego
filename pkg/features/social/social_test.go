package social

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

func TestSocialFeatures(t *testing.T) {
	mockResponse := `for (;;);{"payload":{"success":true},"data":{"viewer":{"user":{"id":"10001"}}}}`

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

	// ChangeBio
	if res, err := ChangeBio(context.Background(), client, fb, "New Bio"); err != nil || res == nil {
		t.Fatalf("ChangeBio failed: %v", err)
	}

	// ToggleProfessionalMode
	if res, err := ToggleProfessionalMode(context.Background(), client, fb, true); err != nil || res == nil {
		t.Fatalf("ToggleProfessionalMode failed: %v", err)
	}

	// BlockUser
	if res, err := BlockUser(context.Background(), client, fb, "bad_user", true); err != nil || res == nil {
		t.Fatalf("BlockUser failed: %v", err)
	}

	// UnfriendUser
	if res, err := UnfriendUser(context.Background(), client, fb, "ex_friend"); err != nil || res == nil {
		t.Fatalf("UnfriendUser failed: %v", err)
	}

	// CreatePost
	if res, err := CreatePost(context.Background(), client, fb, "Hello Facebook from Go!"); err != nil || res == nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	// ReactPost
	if res, err := ReactPost(context.Background(), client, fb, "post_123", "LIKE", false); err != nil || res == nil {
		t.Fatalf("ReactPost failed: %v", err)
	}

	// ArchivePost
	if res, err := ArchivePost(context.Background(), client, fb, "post_123"); err != nil || res == nil {
		t.Fatalf("ArchivePost failed: %v", err)
	}

	// DeletePost
	if res, err := DeletePost(context.Background(), client, fb, "post_123"); err != nil || res == nil {
		t.Fatalf("DeletePost failed: %v", err)
	}

	// SearchUsers
	if res, err := SearchUsers(context.Background(), client, fb, "John Doe"); err != nil {
		t.Fatalf("SearchUsers failed: %v", err)
	} else if res == nil {
		t.Fatalf("expected non-nil results slice")
	}

	// GetNotifications
	if res, err := GetNotifications(context.Background(), client, fb); err != nil || res == nil {
		t.Fatalf("GetNotifications failed: %v", err)
	}
}
