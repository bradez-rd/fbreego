package notes

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

func TestCreateNote(t *testing.T) {
	mockResponse := `for (;;);{"data":{"messenger_note_create":{"note_id":"note_123"}}}`

	client := &http.Client{
		Transport: mockTripper(func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			if !bytes.Contains(body, []byte("MessengerNotesCreateMutation")) {
				t.Errorf("missing doc/mutation in body: %s", string(body))
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(mockResponse)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	fb := &types.DataFB{FacebookID: "10001"}
	res, err := CreateNote(context.Background(), client, fb, "Testing note", "FRIENDS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestDeleteNote(t *testing.T) {
	mockResponse := `for (;;);{"data":{"messenger_note_delete":{"success":true}}}`

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
	res, err := DeleteNote(context.Background(), client, fb, "note_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil response")
	}
}
