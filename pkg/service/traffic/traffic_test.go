package traffic

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockTripper func(req *http.Request) (*http.Response, error)

func (m mockTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}

func TestTrafficStoreAndOutboundTransport(t *testing.T) {
	store := NewTrafficStore(10)

	var hookCalled bool
	store.SetOnRecordHook(func(r *TrafficRecord) {
		hookCalled = true
	})

	baseTransport := mockTripper(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), "sample=data") {
			t.Errorf("expected sample=data in body, got %s", string(body))
		}
		return &http.Response{
			StatusCode: 200,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewBufferString(`for (;;);{"status":"ok"}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	client := &http.Client{
		Transport: NewTrafficTransport(baseTransport, store),
	}

	req, err := http.NewRequest("POST", "https://www.facebook.com/api/graphql/", strings.NewReader("sample=data&c_user=12345"))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Cookie", "c_user=12345; xs=secret;")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if !hookCalled {
		t.Errorf("expected hook to be called on record")
	}

	records := store.List("OUTBOUND", "", 10)
	if len(records) != 1 {
		t.Fatalf("expected 1 record in store, got %d", len(records))
	}

	rec := records[0]
	if rec.Method != "POST" || rec.Target != "Facebook" {
		t.Errorf("unexpected record: %+v", rec)
	}
	if rec.StatusCode != 200 {
		t.Errorf("expected 200, got %d", rec.StatusCode)
	}
	// Cookies must be redacted in RequestHeaders
	if strings.Contains(rec.RequestHeaders["Cookie"], "12345") {
		t.Errorf("sensitive cookie was not redacted in headers: %s", rec.RequestHeaders["Cookie"])
	}
	// Body should have c_user redacted
	if strings.Contains(rec.RequestBody, "c_user=12345") {
		t.Errorf("sensitive cookie was not redacted in body: %s", rec.RequestBody)
	}
	// Response body should have cleaned prefix
	if strings.HasPrefix(rec.ResponseBodyFormat, "for (;;);") {
		t.Errorf("prefix was not stripped in formatted response: %s", rec.ResponseBodyFormat)
	}

	stats := store.Stats()
	if stats.OutboundFB != 1 || stats.TotalRequests != 1 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}

func TestInboundTrafficMiddleware(t *testing.T) {
	store := NewTrafficStore(10)
	middleware := InboundTrafficMiddleware(store)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"code":"SUCCESS","results":{"id":"123"}}`))
	}))

	req := httptest.NewRequest("POST", "/api/v1/sessions", strings.NewReader(`{"label":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	records := store.List("INBOUND", "", 10)
	if len(records) != 1 {
		t.Fatalf("expected 1 inbound record, got %d", len(records))
	}

	rec := records[0]
	if rec.Direction != DirectionInbound || rec.Method != "POST" || rec.StatusCode != 201 {
		t.Errorf("unexpected inbound record: %+v", rec)
	}
}
