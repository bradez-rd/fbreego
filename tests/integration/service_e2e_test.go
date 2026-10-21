package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/service/api"
	"github.com/bradez-rd/fbreego/pkg/service/dispatcher"
	"github.com/bradez-rd/fbreego/pkg/service/events"
	"github.com/bradez-rd/fbreego/pkg/service/media"
	"github.com/bradez-rd/fbreego/pkg/service/sessions"
	"github.com/bradez-rd/fbreego/pkg/service/webhook"
	"github.com/bradez-rd/fbreego/tests/testutil"
)

type mockRewriteTransport struct {
	targetURL *url.URL
}

func (m *mockRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = m.targetURL.Scheme
	req.URL.Host = m.targetURL.Host
	return http.DefaultTransport.RoundTrip(req)
}

func setupTestServer(t *testing.T) (*httptest.Server, *testutil.MockFacebookServer, func()) {
	tmpDir, err := os.MkdirTemp("", "fbreego_e2e_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	prevAuth := os.Getenv("FBCHAT_API_AUTH_REQUIRED")
	_ = os.Setenv("FBCHAT_API_AUTH_REQUIRED", "0")

	fbMock := testutil.NewMockFacebookServer()
	mockTarget, _ := url.Parse(fbMock.URL)

	mockHTTPClient := &http.Client{
		Transport: &mockRewriteTransport{targetURL: mockTarget},
	}

	encStore := sessions.NewEncryptedSessionStore(tmpDir)
	webhookStore := webhook.NewWebhookConfigStore(tmpDir)
	webhookDisp := dispatcher.NewDispatcher(webhookStore)
	eventHub := events.NewRealtimeHub()
	listenerMgr := events.NewSessionListenerManager(eventHub)
	mediaStore := media.NewMediaStorage(tmpDir + "/media")
	sessionReg := sessions.NewSessionRegistry(encStore, mockHTTPClient)

	ctx, cancel := context.WithCancel(context.Background())
	webhookDisp.Start(ctx)

	server := api.NewServer(
		sessionReg,
		webhookStore,
		webhookDisp,
		mediaStore,
		eventHub,
		listenerMgr,
	)

	ts := httptest.NewServer(server.Routes())

	cleanup := func() {
		cancel()
		listenerMgr.StopAll()
		webhookDisp.Stop()
		ts.Close()
		fbMock.Close()
		_ = os.RemoveAll(tmpDir)
		_ = os.Setenv("FBCHAT_API_AUTH_REQUIRED", prevAuth)
	}

	return ts, fbMock, cleanup
}

func TestE2E_FullFlow(t *testing.T) {
	ts, _, cleanup := setupTestServer(t)
	defer cleanup()

	client := ts.Client()

	// 1. Healthz
	t.Run("GET /healthz", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/healthz")
		if err != nil {
			t.Fatalf("healthz request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	// 2. Readyz
	t.Run("GET /readyz", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/readyz")
		if err != nil {
			t.Fatalf("readyz request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	// 3. Diagnostics
	t.Run("GET /api/v1/diagnostics", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/v1/diagnostics")
		if err != nil {
			t.Fatalf("diagnostics request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	// 4. Portal Web SPA
	t.Run("GET / (portal)", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/")
		if err != nil {
			t.Fatalf("portal request failed: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if !bytes.Contains(body, []byte("fbchat_v2")) && !bytes.Contains(body, []byte("fbreego")) && !bytes.Contains(body, []byte("html")) {
			t.Errorf("portal response does not look like valid HTML")
		}
	})

	// 5. OpenAPI schema
	t.Run("GET /openapi.json", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/openapi.json")
		if err != nil {
			t.Fatalf("openapi request failed: %v", err)
		}
		defer resp.Body.Close()
		var schema map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&schema); err != nil {
			t.Fatalf("failed to decode openapi schema: %v", err)
		}
		if schema["openapi"] != "3.0.3" {
			t.Errorf("expected OpenAPI 3.0.3, got %v", schema["openapi"])
		}
	})

	// 6. Session Management Lifecycle
	var createdSessionID string
	t.Run("POST /api/v1/sessions & Session Lifecycle", func(t *testing.T) {
		createPayload := types.SessionCreateInput{
			Label:    "e2e_test_session",
			TenantID: "default",
			Cookies: []types.CookieInput{
				{Name: "c_user", Value: "1000888999"},
				{Name: "xs", Value: "secure_token_abc"},
			},
		}
		raw, _ := json.Marshal(createPayload)
		resp, err := client.Post(ts.URL+"/api/v1/sessions", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("create session failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200/201, got %d. Body: %s", resp.StatusCode, string(body))
		}

		var createResp map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&createResp)
		if createResp["code"] != "SUCCESS" {
			t.Errorf("expected SUCCESS code, got %v", createResp["code"])
		}

		results, _ := createResp["results"].(map[string]any)
		if results != nil {
			createdSessionID, _ = results["sessionId"].(string)
		}
		if createdSessionID == "" {
			t.Fatalf("missing sessionId in create response: %v", createResp)
		}

		// GET /api/v1/sessions
		listResp, err := client.Get(ts.URL + "/api/v1/sessions")
		if err != nil {
			t.Fatalf("list sessions failed: %v", err)
		}
		defer listResp.Body.Close()
		var listData map[string]any
		_ = json.NewDecoder(listResp.Body).Decode(&listData)
		if listData["code"] != "SUCCESS" {
			t.Errorf("expected SUCCESS code in list: %v", listData)
		}

		// GET /api/v1/sessions/{id}/status
		statusResp, err := client.Get(ts.URL + "/api/v1/sessions/" + createdSessionID + "/status")
		if err != nil {
			t.Fatalf("session status failed: %v", err)
		}
		defer statusResp.Body.Close()
		if statusResp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 for session status, got %d", statusResp.StatusCode)
		}

		// DELETE /api/v1/sessions/{id}
		req, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/sessions/"+createdSessionID, nil)
		delResp, err := client.Do(req)
		if err != nil {
			t.Fatalf("delete session failed: %v", err)
		}
		defer delResp.Body.Close()
		if delResp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 on delete, got %d", delResp.StatusCode)
		}
	})

	// 7. Webhook Configuration
	t.Run("Webhook Config & Testing", func(t *testing.T) {
		// GET initial webhook config
		resp, err := client.Get(ts.URL + "/api/v1/webhook")
		if err != nil {
			t.Fatalf("get webhook failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		// PUT update webhook config
		secret := "my_e2e_secret"
		updatePayload := types.WebhookConfigInput{
			Enabled: true,
			URL:     "https://httpbin.org/post",
			Events:  []string{"message", "session_status"},
			Secret:  &secret,
		}
		raw, _ := json.Marshal(updatePayload)
		putReq, _ := http.NewRequest("PUT", ts.URL+"/api/v1/webhook", bytes.NewReader(raw))
		putReq.Header.Set("Content-Type", "application/json")
		putResp, err := client.Do(putReq)
		if err != nil {
			t.Fatalf("update webhook failed: %v", err)
		}
		defer putResp.Body.Close()
		if putResp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 on webhook update, got %d", putResp.StatusCode)
		}

		// POST test webhook
		testPayload := map[string]string{
			"url": "https://httpbin.org/post",
		}
		testRaw, _ := json.Marshal(testPayload)
		testResp, err := client.Post(ts.URL+"/api/v1/webhook/test", "application/json", bytes.NewReader(testRaw))
		if err != nil {
			t.Fatalf("test webhook endpoint failed: %v", err)
		}
		defer testResp.Body.Close()
		if testResp.StatusCode != http.StatusOK && testResp.StatusCode != http.StatusBadGateway {
			t.Errorf("unexpected status code: %d", testResp.StatusCode)
		}
	})

	// 8. Media Storage API
	t.Run("Media Upload & Retrieval", func(t *testing.T) {
		var b bytes.Buffer
		w := multipart.NewWriter(&b)
		part, err := w.CreateFormFile("file", "sample_image.png")
		if err != nil {
			t.Fatalf("create form file failed: %v", err)
		}
		_, _ = part.Write([]byte("fake png binary stream for e2e test"))
		_ = w.Close()

		uploadResp, err := client.Post(ts.URL+"/api/v1/media/upload", w.FormDataContentType(), &b)
		if err != nil {
			t.Fatalf("media upload failed: %v", err)
		}
		defer uploadResp.Body.Close()
		if uploadResp.StatusCode != http.StatusOK && uploadResp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 200/201 on media upload, got %d", uploadResp.StatusCode)
		}

		var uploadData map[string]any
		_ = json.NewDecoder(uploadResp.Body).Decode(&uploadData)
		results, _ := uploadData["results"].(map[string]any)
		if results == nil {
			t.Fatalf("missing media results: %v", uploadData)
		}
		mediaID, _ := results["mediaId"].(string)
		if mediaID == "" {
			t.Fatalf("expected mediaId, got empty")
		}

		// GET media metadata
		getResp, err := client.Get(ts.URL + "/api/v1/media/" + mediaID)
		if err != nil {
			t.Fatalf("get media failed: %v", err)
		}
		defer getResp.Body.Close()
		if getResp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", getResp.StatusCode)
		}

		// GET media list
		listResp, err := client.Get(ts.URL + "/api/v1/media")
		if err != nil {
			t.Fatalf("list media failed: %v", err)
		}
		defer listResp.Body.Close()
		if listResp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", listResp.StatusCode)
		}
	})

	// 9. WebSocket Ticket Generation
	t.Run("POST /api/v1/websocket-ticket", func(t *testing.T) {
		ticketPayload := map[string]string{
			"sessionId": "session_abc",
		}
		raw, _ := json.Marshal(ticketPayload)
		resp, err := client.Post(ts.URL+"/api/v1/websocket-ticket", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("ws ticket request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			t.Errorf("expected 200/201, got %d", resp.StatusCode)
		}

		var ticketResp map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&ticketResp)
		if ticketResp["code"] != "SUCCESS" {
			t.Errorf("expected SUCCESS code: %v", ticketResp)
		}
		results, _ := ticketResp["results"].(map[string]any)
		if results == nil || results["ticket"] == nil {
			t.Errorf("missing ticket in results: %v", results)
		}
	})
}
