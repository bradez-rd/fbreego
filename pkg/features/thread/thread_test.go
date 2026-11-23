package thread

import (
	"bytes"
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

func TestFetchThreadPageCarriesCursorAndProviderPagination(t *testing.T) {
	client := &http.Client{Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		var query map[string]map[string]any
		if err := json.Unmarshal([]byte(req.Form.Get("queries")), &query); err != nil {
			t.Fatal(err)
		}
		params := query["o0"]["query_params"].(map[string]any)
		if params["limit"] != float64(1) || params["before"] != float64(200) {
			t.Fatalf("unexpected pagination parameters: %#v", params)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"o0":{"data":{"viewer":{"message_threads":{"nodes":[{"updated_time_precise":"100"}],"page_info":{"has_next_page":true}}}}}}`)), Header: make(http.Header)}, nil
	})}
	data, err := FetchThreadPage(context.Background(), client, &types.DataFB{}, 1, "200")
	if err != nil {
		t.Fatal(err)
	}
	more, next := ThreadPageInfo(data, 1)
	if more == nil || !*more || next != "100" {
		t.Fatalf("unexpected page info: %v, %q", more, next)
	}
}

func TestThreadPageInfoDoesNotInventCompletion(t *testing.T) {
	data := map[string]any{"data": map[string]any{"viewer": map[string]any{"message_threads": map[string]any{"nodes": []any{}}}}}
	more, cursor := ThreadPageInfo(data, 1)
	if more != nil || cursor != "" {
		t.Fatalf("missing provider metadata must remain unknown: %v, %q", more, cursor)
	}
	data["data"].(map[string]any)["viewer"].(map[string]any)["message_threads"].(map[string]any)["nodes"] = []any{map[string]any{"updated_time_precise": "100"}}
	more, cursor = ThreadPageInfo(data, 1)
	if more != nil || cursor != "100" {
		t.Fatalf("full page must offer timestamp without claiming completion: %v, %q", more, cursor)
	}
	data["data"].(map[string]any)["viewer"].(map[string]any)["message_threads"].(map[string]any)["page_info"] = map[string]any{"has_next_page": false}
	more, cursor = ThreadPageInfo(data, 1)
	if more == nil || *more || cursor != "" {
		t.Fatalf("provider end-of-page not respected: %v, %q", more, cursor)
	}
}

func TestFetchThreadMessagesPageExposesResumeBoundaryWithoutClaimingCompletion(t *testing.T) {
	client := &http.Client{Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(req.Form.Get("queries"), `"load_messages_before_time":200`) {
			t.Fatalf("missing before boundary: %s", req.Form.Get("queries"))
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"o0":{"data":{"message_thread":{"messages":{"nodes":[{"message_id":"m1","timestamp_precise":"100"}]}}}}}`)), Header: make(http.Header)}, nil
	})}
	page, err := FetchThreadMessagesPage(context.Background(), client, &types.DataFB{}, "thread", 1, "200")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.HasMore != nil || page.NextBefore != "100" {
		t.Fatalf("unexpected message page: %#v", page)
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
