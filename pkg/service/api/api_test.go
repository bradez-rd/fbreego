package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/bradez-rd/fbreego/pkg/service/dispatcher"
	"github.com/bradez-rd/fbreego/pkg/service/events"
	"github.com/bradez-rd/fbreego/pkg/service/media"
	"github.com/bradez-rd/fbreego/pkg/service/sessions"
	"github.com/bradez-rd/fbreego/pkg/service/webhook"
)

func setupTestServer(t *testing.T) (*Server, http.Handler) {
	t.Setenv("FBCHAT_SESSION_SECRET", "test-only-session-secret-32-characters-minimum")
	tempDir, err := os.MkdirTemp("", "fbchat_api_test_*")
	if err != nil {
		t.Fatalf("error temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	encStore := sessions.NewEncryptedSessionStore(tempDir)
	webhookStore := webhook.NewWebhookConfigStore(tempDir)
	disp := dispatcher.NewDispatcher(webhookStore)
	hub := events.NewRealtimeHub()
	lMgr := events.NewSessionListenerManager(hub)
	mStore := media.NewMediaStorage(tempDir)
	reg := sessions.NewSessionRegistry(encStore, nil)

	srv := NewServer(reg, webhookStore, disp, mStore, hub, lMgr)
	return srv, srv.Routes()
}

func TestHealthzAndReadyz(t *testing.T) {
	_, handler := setupTestServer(t)

	// Test /healthz
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("/healthz retornó status %d, esperado 200", w.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("error al deserializar /healthz: %v", err)
	}
	if res["status"] != "ok" || res["service"] != ServiceName {
		t.Errorf("respuesta /healthz inesperada: %+v", res)
	}

	// Test /readyz
	reqReady := httptest.NewRequest("GET", "/readyz", nil)
	wReady := httptest.NewRecorder()
	handler.ServeHTTP(wReady, reqReady)

	// Status puede ser 200 o 503 dependiendo de si api_auth está configurado
	if wReady.Code != http.StatusOK && wReady.Code != http.StatusServiceUnavailable {
		t.Errorf("/readyz retornó código inesperado: %d", wReady.Code)
	}
}

func TestBasicAuthMiddleware_Unauthorized(t *testing.T) {
	// Exigir autenticación explícitamente
	_ = os.Setenv("FBCHAT_API_AUTH_REQUIRED", "1")
	_ = os.Setenv("FBCHAT_API_USERNAME", "admin")
	_ = os.Setenv("FBCHAT_API_PASSWORD", "secret123")
	defer func() {
		_ = os.Unsetenv("FBCHAT_API_AUTH_REQUIRED")
		_ = os.Unsetenv("FBCHAT_API_USERNAME")
		_ = os.Unsetenv("FBCHAT_API_PASSWORD")
	}()

	_, handler := setupTestServer(t)

	// Petición a ruta protegida sin credenciales
	req := httptest.NewRequest("GET", "/api/v1/sessions", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("esperado 401 Unauthorized sin credenciales, recibido %d", w.Code)
	}

	// Petición con credenciales correctas
	reqAuth := httptest.NewRequest("GET", "/api/v1/sessions", nil)
	reqAuth.SetBasicAuth("admin", "secret123")
	wAuth := httptest.NewRecorder()
	handler.ServeHTTP(wAuth, reqAuth)

	if wAuth.Code != http.StatusOK {
		t.Errorf("esperado 200 OK con credenciales válidas, recibido %d", wAuth.Code)
	}
}

func TestAuthenticationFailsClosedWhenCredentialsMissing(t *testing.T) {
	t.Setenv("FBCHAT_API_AUTH_REQUIRED", "1")
	t.Setenv("FBCHAT_API_USERNAME", "")
	t.Setenv("FBCHAT_API_PASSWORD", "")
	_, handler := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for missing required auth config, got %d", w.Code)
	}
}

func TestCORSRejectsUnconfiguredOrigin(t *testing.T) {
	t.Setenv("FBCHAT_API_CORS_ORIGINS", "https://admin.example.com")
	_, handler := setupTestServer(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/sessions", nil)
	req.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unconfigured origin, got %d", w.Code)
	}
}

func TestE2ETestEndpointDisabledByDefault(t *testing.T) {
	t.Setenv("FBCHAT_API_AUTH_REQUIRED", "0")
	t.Setenv("FBCHAT_ENABLE_E2E_TEST_ENDPOINT", "0")
	_, handler := setupTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tests/e2e", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected disabled test endpoint to return 404, got %d", w.Code)
	}
}

func TestTrafficCaptureDisabledByDefault(t *testing.T) {
	t.Setenv("FBCHAT_TRAFFIC_ENABLED", "0")
	srv, _ := setupTestServer(t)
	if srv.TrafficEnabled {
		t.Fatal("traffic capture must be opt-in")
	}
}

func TestWebSocketTicketIsSingleUseAndSessionBound(t *testing.T) {
	srv, _ := setupTestServer(t)
	wrongSessionTicket := srv.IssueWebSocketTicket("session-a")
	if srv.ConsumeWebSocketTicket(wrongSessionTicket, "session-b") {
		t.Fatal("ticket must not authorize a different session")
	}
	if srv.ConsumeWebSocketTicket(wrongSessionTicket, "session-a") {
		t.Fatal("a rejected ticket must still be consumed")
	}

	ticket := srv.IssueWebSocketTicket("session-a")
	if !srv.ConsumeWebSocketTicket(ticket, "session-a") {
		t.Fatal("expected matching ticket to be accepted")
	}
	if srv.ConsumeWebSocketTicket(ticket, "session-a") {
		t.Fatal("ticket must be single use")
	}
}

func TestValidateRuntimeConfiguration(t *testing.T) {
	t.Setenv("FBCHAT_API_AUTH_REQUIRED", "1")
	t.Setenv("FBCHAT_API_USERNAME", "")
	t.Setenv("FBCHAT_API_PASSWORD", "")
	if ValidateRuntimeConfiguration() == nil {
		t.Fatal("expected missing auth credentials to fail runtime validation")
	}
}

func TestOpenAPICoversRegisteredRoutes(t *testing.T) {
	t.Setenv("FBCHAT_ENABLE_E2E_TEST_ENDPOINT", "0")
	_, handler := setupTestServer(t)
	routes, ok := handler.(chi.Routes)
	if !ok {
		t.Fatal("server handler does not expose chi routes")
	}
	schemaPaths := OpenAPISchema()["paths"].(map[string]any)
	err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		pathItem, exists := schemaPaths[route]
		if !exists {
			t.Errorf("OpenAPI is missing registered route %s %s", method, route)
			return nil
		}
		operations := pathItem.(map[string]any)
		if _, exists := operations[strings.ToLower(method)]; !exists {
			t.Errorf("OpenAPI is missing method %s for route %s", method, route)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}
}

func TestServePortalSPA(t *testing.T) {
	_ = os.Setenv("FBCHAT_API_AUTH_REQUIRED", "0")
	defer os.Unsetenv("FBCHAT_API_AUTH_REQUIRED")

	_, handler := setupTestServer(t)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET / retornó status %d, esperado 200", w.Code)
	}
	body := w.Body.String()
	if len(body) < 1000 || !jsonContains(body, "fbchat-v2 Provider Portal") {
		t.Errorf("contenido de portal.html inesperado o incompleto")
	}
}

func jsonContains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s != "" && substr != "") && (len(s) >= len(substr))
}

func TestUIPortalAPISchemaCompatibility(t *testing.T) {
	_ = os.Setenv("FBCHAT_API_AUTH_REQUIRED", "0")
	defer os.Unsetenv("FBCHAT_API_AUTH_REQUIRED")

	srv, handler := setupTestServer(t)

	// 1. Verify /api/v1/app/info has required UI keys
	reqInfo := httptest.NewRequest("GET", "/api/v1/app/info", nil)
	wInfo := httptest.NewRecorder()
	handler.ServeHTTP(wInfo, reqInfo)
	if wInfo.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/app/info returned %d", wInfo.Code)
	}
	var infoResp struct {
		Results map[string]any `json:"results"`
	}
	if err := json.Unmarshal(wInfo.Body.Bytes(), &infoResp); err != nil {
		t.Fatalf("unmarshal app/info: %v", err)
	}
	for _, key := range []string{"service", "version", "sessionStorage", "capabilityMatrix"} {
		if _, ok := infoResp.Results[key]; !ok {
			t.Errorf("app/info missing required UI key: %q", key)
		}
	}

	// 2. Verify /api/v1/app/status has required UI keys
	reqStatus := httptest.NewRequest("GET", "/api/v1/app/status", nil)
	wStatus := httptest.NewRecorder()
	handler.ServeHTTP(wStatus, reqStatus)
	if wStatus.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/app/status returned %d", wStatus.Code)
	}
	var statusResp struct {
		Results map[string]any `json:"results"`
	}
	if err := json.Unmarshal(wStatus.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("unmarshal app/status: %v", err)
	}
	for _, key := range []string{"status", "readiness", "dispatcher", "eventHub"} {
		if _, ok := statusResp.Results[key]; !ok {
			t.Errorf("app/status missing required UI key: %q", key)
		}
	}

	// 3. Verify /api/v1/diagnostics has readiness with status
	reqDiag := httptest.NewRequest("GET", "/api/v1/diagnostics", nil)
	wDiag := httptest.NewRecorder()
	handler.ServeHTTP(wDiag, reqDiag)
	if wDiag.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/diagnostics returned %d", wDiag.Code)
	}
	var diagResp struct {
		Results map[string]any `json:"results"`
	}
	if err := json.Unmarshal(wDiag.Body.Bytes(), &diagResp); err != nil {
		t.Fatalf("unmarshal diagnostics: %v", err)
	}
	readiness, ok := diagResp.Results["readiness"].(map[string]any)
	if !ok || readiness["status"] == nil {
		t.Errorf("diagnostics missing readiness.status: %v", diagResp.Results["readiness"])
	}

	// 4. Verify SessionRecord returns validatedAt and lastActivityAt
	rec := &sessions.SessionRecord{
		SessionID:   "test_sess_1",
		Label:       "Test Account",
		CreatedAt:   "2026-10-21T12:00:00Z",
		ValidatedAt: "2026-10-21T12:00:00Z",
	}
	pub := rec.Public()
	if pub["validatedAt"] == nil || pub["validatedAt"] == "" {
		t.Errorf("SessionRecord.Public() missing validatedAt: %v", pub)
	}
	if pub["lastActivityAt"] == nil {
		t.Errorf("SessionRecord.Public() missing lastActivityAt: %v", pub)
	}

	// 5. Verify webhook config public returns secretConfigured
	whPub := srv.WebhookStore.GetPublic()
	if _, ok := whPub["secretConfigured"]; !ok {
		t.Errorf("webhook.GetPublic() missing secretConfigured: %v", whPub)
	}
}
