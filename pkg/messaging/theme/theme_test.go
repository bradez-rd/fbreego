package theme

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

func TestListThemes(t *testing.T) {
	mockResponse := `for (;;);{"data":{"messenger_thread_themes":{"theme_edges":[{"node":{"theme_id":"theme_1","theme_name":"Ocean","accessibility_label":"Ocean Blue","theme_key":"ocean"}}]}}}`

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
	themes, err := ListThemes(context.Background(), client, fb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(themes) != 1 {
		t.Fatalf("expected 1 theme, got %d", len(themes))
	}
	if themes[0].Name != "Ocean" || themes[0].ID != "theme_1" {
		t.Errorf("unexpected theme data: %+v", themes[0])
	}
}
