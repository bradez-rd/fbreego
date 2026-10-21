package attachments

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bradez-rd/fbreego/pkg/core/types"
)

type mockTripper func(req *http.Request) (*http.Response, error)

func (m mockTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}

func TestUploadImage(t *testing.T) {
	mockResponse := `for (;;);{"payload":{"metadata":[{"image_id":"987654321","image_type":"FILE_ATTACHMENT"}]}}`

	client := &http.Client{
		Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/form-data") {
				t.Errorf("expected multipart Content-Type, got %s", req.Header.Get("Content-Type"))
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(mockResponse)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	fb := &types.DataFB{FacebookID: "10001"}
	fileContent := bytes.NewReader([]byte("fake image data"))

	res, err := Upload(context.Background(), client, fb, "test.png", fileContent, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.AttachmentID != "987654321" {
		t.Errorf("expected attachment ID 987654321, got %s", res.AttachmentID)
	}
	if res.AttachmentType != "image" {
		t.Errorf("expected attachment type 'image', got %s", res.AttachmentType)
	}
}
