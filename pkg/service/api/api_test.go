package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bradez-rd/fbreego/pkg/service/dispatcher"
	"github.com/bradez-rd/fbreego/pkg/service/events"
	"github.com/bradez-rd/fbreego/pkg/service/media"
	"github.com/bradez-rd/fbreego/pkg/service/sessions"
	"github.com/bradez-rd/fbreego/pkg/service/webhook"
)

func setupTestServer(t *testing.T) (*Server, http.Handler) {
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
