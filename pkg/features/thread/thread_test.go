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

func TestParseGraphQLMessageNodeNormalizesHistoricalImage(t *testing.T) {
	message := parseGraphQLMessageNode(map[string]any{
		"message_id":        "mid.image.1",
		"message_sender":    map[string]any{"id": "user-2"},
		"message":           map[string]any{"text": ""},
		"timestamp_precise": "1793847485447",
		"blob_attachments": []any{map[string]any{
			"__typename":           "MessageImage",
			"legacy_attachment_id": "attachment-1",
			"filename":             "image-1",
			"original_extension":   "jpg",
			"thumbnail":            map[string]any{"uri": "https://cdn.example/thumb.jpg"},
			"preview":              map[string]any{"uri": "https://cdn.example/preview.jpg"},
			"large_preview":        map[string]any{"uri": "https://cdn.example/large.jpg"},
		}},
	}, "thread-1", "own-user")

	if len(message.Attachments) != 1 {
		t.Fatalf("expected one image attachment, got %#v", message.Attachments)
	}
	attachment := message.Attachments[0]
	if attachment["type"] != "image" || attachment["url"] != "https://cdn.example/large.jpg" {
		t.Fatalf("unexpected canonical image attachment: %#v", attachment)
	}
	if attachment["fileName"] != "image-1.jpg" || attachment["mimeType"] != "image/jpeg" {
		t.Fatalf("missing canonical image metadata: %#v", attachment)
	}
}

func TestParseGraphQLMessageNodePrefersStickerAndNormalizesLinkFallback(t *testing.T) {
	sticker := parseGraphQLMessageNode(map[string]any{
		"message_id":     "mid.sticker.1",
		"message_sender": map[string]any{"id": "user-2"},
		"sticker": map[string]any{
			"id":           "sticker-1",
			"url":          "https://cdn.example/sticker.webp",
			"sprite_image": "https://cdn.example/sticker-preview.webp",
		},
		"blob_attachments": []any{map[string]any{
			"__typename": "MessageFile",
			"url":        "https://cdn.example/ignored.pdf",
		}},
	}, "thread-1", "own-user")
	if len(sticker.Attachments) != 1 || sticker.Attachments[0]["type"] != "sticker" {
		t.Fatalf("expected sticker to take priority: %#v", sticker.Attachments)
	}

	link := parseGraphQLMessageNode(map[string]any{
		"message_id":     "mid.link.1",
		"message_sender": map[string]any{"id": "user-2"},
		"extensible_attachment": map[string]any{
			"legacy_attachment_id": "share-1",
			"story_attachment": map[string]any{
				"url":                 "https://marketplace.example/item/1",
				"title_with_entities": map[string]any{"text": "Producto publicado"},
				"description":         map[string]any{"text": "Descripción"},
				"media":               map[string]any{"image": map[string]any{"uri": "https://cdn.example/item.jpg"}},
			},
		},
	}, "thread-1", "own-user")
	if len(link.Attachments) != 1 || link.Attachments[0]["type"] != "link" {
		t.Fatalf("expected canonical link attachment: %#v", link.Attachments)
	}
	if link.Text != "Producto publicado" {
		t.Fatalf("expected link title fallback, got %q", link.Text)
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
