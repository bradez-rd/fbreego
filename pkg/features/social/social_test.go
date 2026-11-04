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

func TestMarketplaceFeatures(t *testing.T) {
	mockSearchResponse := `for (;;);{"data":{"marketplace_search":{"feed_units":{"edges":[{"node":{"listing":{"id":"999123","marketplace_listing_title":"Laptop ThinkPad X1","listing_price":{"formatted_amount":"$450"},"location":{"reverse_geocode":{"city_page":{"display_name":"Santo Domingo"}}},"primary_listing_photo":{"image":{"uri":"https://fbcdn.net/p999.jpg"}},"marketplace_listing_seller":{"name":"Alice"},"creation_time":1700000000,"is_pending":false}}}],"page_info":{"has_next_page":true,"end_cursor":"cur_123"}}}}}`

	client := &http.Client{
		Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(mockSearchResponse)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	fb := &types.DataFB{FacebookID: "10001"}

	// 1. Resolve Category
	catID := ResolveCategoryID("Electronics & computers")
	if catID != 1792291877663080 {
		t.Fatalf("expected 1792291877663080, got %d", catID)
	}
	catFallback := ResolveCategoryID("UnknownCategory123")
	if catFallback != 1792291877663080 {
		t.Fatalf("expected fallback category ID, got %d", catFallback)
	}

	// 2. SearchMarketplace
	searchRes, err := SearchMarketplace(context.Background(), client, fb, MarketplaceSearchParams{
		Query: "laptop",
	})
	if err != nil {
		t.Fatalf("SearchMarketplace failed: %v", err)
	}
	if len(searchRes.Listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(searchRes.Listings))
	}
	item := searchRes.Listings[0]
	if item.ID != "999123" || item.Title != "Laptop ThinkPad X1" || item.Price != "$450" || item.Location != "Santo Domingo" {
		t.Fatalf("unexpected item fields: %+v", item)
	}
	if !searchRes.HasNextPage || searchRes.EndCursor != "cur_123" {
		t.Fatalf("unexpected pagination: %+v", searchRes)
	}

	// 3. CreateMarketplaceItem
	createRes, err := CreateMarketplaceItem(context.Background(), client, fb, MarketplaceItemInput{
		Title:       "Licencia Software ERP Pro",
		Price:       99,
		Currency:    "USD",
		Description: "Software de gestión empresarial para pymes",
		Category:    "Electronics & computers",
		ImageIDs:    []string{"photo_888123"},
	})
	if err != nil {
		t.Fatalf("CreateMarketplaceItem failed: %v", err)
	}
	if createRes == nil {
		t.Fatalf("expected non-nil response from CreateMarketplaceItem")
	}
}
