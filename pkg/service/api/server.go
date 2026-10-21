package api

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bradez-rd/fbreego/pkg/messaging/e2ee"
	"github.com/bradez-rd/fbreego/pkg/service/dispatcher"
	"github.com/bradez-rd/fbreego/pkg/service/events"
	"github.com/bradez-rd/fbreego/pkg/service/media"
	"github.com/bradez-rd/fbreego/pkg/service/sessions"
	"github.com/bradez-rd/fbreego/pkg/service/traffic"
	"github.com/bradez-rd/fbreego/pkg/service/webhook"
)

const (
	ServiceName            = "fbchat-provider"
	Version                = "2.3.3"
	WebSocketTicketTTLSecs = 30
)

// AuditLogEntry representa un registro de auditoría en memoria.
type AuditLogEntry struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Details   string `json:"details"`
	SessionID string `json:"sessionId,omitempty"`
}

type websocketTicket struct {
	ExpiresAt time.Time
	SessionID string
}

// Server estructura los componentes principales del microservicio HTTP REST y WebSocket.
type Server struct {
	Registry        *sessions.SessionRegistry
	WebhookStore    *webhook.WebhookConfigStore
	Dispatcher      *dispatcher.Dispatcher
	MediaStore      *media.MediaStorage
	EventHub        *events.RealtimeHub
	ListenerManager *events.SessionListenerManager
	TrafficStore    *traffic.TrafficStore
	TrafficEnabled  bool
	E2EEManager     *e2ee.ProtocolManager
	httpClient      *http.Client

	ticketsMu sync.Mutex
	tickets   map[string]websocketTicket

	auditMu   sync.Mutex
	auditLogs []AuditLogEntry
}

// NewServer inicializa los subsistemas del microservicio.
func NewServer(
	reg *sessions.SessionRegistry,
	wsStore *webhook.WebhookConfigStore,
	disp *dispatcher.Dispatcher,
	mStore *media.MediaStorage,
	hub *events.RealtimeHub,
	lManager *events.SessionListenerManager,
) *Server {
	if wsStore == nil {
		wsStore = webhook.NewWebhookConfigStore("")
	}
	if disp == nil {
		disp = dispatcher.NewDispatcher(wsStore)
	}
	if hub == nil {
		hub = events.NewRealtimeHub()
	}
	hub.SetDispatcher(disp)
	if lManager == nil {
		lManager = events.NewSessionListenerManager(hub)
	}
	if mStore == nil {
		mStore = media.NewMediaStorage("")
	}
	if reg == nil {
		reg = sessions.NewSessionRegistry(nil, nil)
	}

	trafficStore := traffic.NewTrafficStore(300)
	if hub != nil {
		trafficStore.SetOnRecordHook(func(r *traffic.TrafficRecord) {
			evt := map[string]any{
				"event":   "traffic_request",
				"request": r,
			}
			if r.SessionID != "" {
				evt["sessionId"] = r.SessionID
			}
			if r.TenantID != "" {
				evt["tenantId"] = r.TenantID
			}
			hub.Broadcast(evt)
		})
	}

	var baseTransport http.RoundTripper = http.DefaultTransport
	timeout := 30 * time.Second
	if reg != nil && reg.HTTPClient() != nil {
		if reg.HTTPClient().Transport != nil {
			baseTransport = reg.HTTPClient().Transport
		}
		if reg.HTTPClient().Timeout > 0 {
			timeout = reg.HTTPClient().Timeout
		}
	}

	trafficEnabled := envFlag("FBCHAT_TRAFFIC_ENABLED", false)
	transport := baseTransport
	if trafficEnabled {
		transport = traffic.NewTrafficTransport(baseTransport, trafficStore)
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
	if reg != nil {
		reg.SetHTTPClient(httpClient)
	}

	return &Server{
		Registry:        reg,
		WebhookStore:    wsStore,
		Dispatcher:      disp,
		MediaStore:      mStore,
		EventHub:        hub,
		ListenerManager: lManager,
		TrafficStore:    trafficStore,
		TrafficEnabled:  trafficEnabled,
		E2EEManager:     e2ee.NewProtocolManager(),
		httpClient:      httpClient,
		tickets:         make(map[string]websocketTicket),
	}
}

// IssueWebSocketTicket emite un token de un solo uso válido por 30 segundos.
func (s *Server) IssueWebSocketTicket(sessionID string) string {
	s.ticketsMu.Lock()
	defer s.ticketsMu.Unlock()

	now := time.Now()
	for t, stored := range s.tickets {
		if now.After(stored.ExpiresAt) {
			delete(s.tickets, t)
		}
	}

	b := make([]byte, 32)
	_, _ = rand.Read(b)
	ticket := base64.RawURLEncoding.EncodeToString(b)
	s.tickets[ticket] = websocketTicket{ExpiresAt: now.Add(WebSocketTicketTTLSecs * time.Second), SessionID: sessionID}
	return ticket
}

// ConsumeWebSocketTicket valida y destruye el ticket si sigue vigente.
func (s *Server) ConsumeWebSocketTicket(ticket, sessionID string) bool {
	if ticket == "" {
		return false
	}
	s.ticketsMu.Lock()
	defer s.ticketsMu.Unlock()

	stored, found := s.tickets[ticket]
	if !found {
		return false
	}
	delete(s.tickets, ticket)
	if stored.SessionID != "" && stored.SessionID != sessionID {
		return false
	}
	return time.Now().Before(stored.ExpiresAt)
}

// RecordAudit registra un evento de auditoría en el buffer circular (últimos 100).
func (s *Server) RecordAudit(eventType, details, sessionID string) {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()

	entry := AuditLogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Type:      eventType,
		Details:   details,
		SessionID: sessionID,
	}
	s.auditLogs = append(s.auditLogs, entry)
	if len(s.auditLogs) > 100 {
		s.auditLogs = s.auditLogs[len(s.auditLogs)-100:]
	}
}

// recordSessionActivity actualiza la actividad sin convertir un fallo de
// persistencia posterior en un falso fallo de una operación ya completada.
func (s *Server) recordSessionActivity(sessionID string) {
	if err := s.Registry.RecordActivity(sessionID); err != nil {
		s.RecordAudit("session_activity_persist_failed", err.Error(), sessionID)
	}
}

// ReadinessReport evalúa el estado de salud y configuración del microservicio.
func (s *Server) ReadinessReport() map[string]any {
	auth := currentAuthConfiguration()
	authReady := !auth.Required || auth.Configured
	sessionStorageReady := s.Registry.Ready() == nil
	mediaStorageReady := s.MediaStore.Ready() == nil

	status := "ready"
	if !authReady || !sessionStorageReady || !mediaStorageReady {
		status = "not_ready"
	}

	return map[string]any{
		"status":  status,
		"service": ServiceName,
		"version": Version,
		"checks": map[string]any{
			"api_auth": map[string]any{
				"ready":      authReady,
				"required":   auth.Required,
				"configured": auth.Configured,
			},
			"session_storage": map[string]any{
				"ready": sessionStorageReady,
			},
			"media_storage": map[string]any{
				"ready": mediaStorageReady,
			},
			"e2ee_bridge": map[string]any{
				"ready":      false,
				"required":   false,
				"configured": false,
			},
			"messenger_listener": map[string]any{
				"ready":      false,
				"required":   false,
				"configured": false,
			},
			"portal": map[string]any{
				"ready": true,
			},
		},
	}
}

// extractTenantID extrae el identificador de inquilino de las cabeceras HTTP o query params.
func extractTenantID(r *http.Request) string {
	if r == nil {
		return ""
	}
	tenant := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if tenant == "" {
		tenant = strings.TrimSpace(r.URL.Query().Get("tenantId"))
	}
	return tenant
}

