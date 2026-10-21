package api

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/bradez-rd/fbreego/pkg/messaging/e2ee"
	"github.com/bradez-rd/fbreego/pkg/service/dispatcher"
	"github.com/bradez-rd/fbreego/pkg/service/events"
	"github.com/bradez-rd/fbreego/pkg/service/media"
	"github.com/bradez-rd/fbreego/pkg/service/sessions"
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

// Server estructura los componentes principales del microservicio HTTP REST y WebSocket.
type Server struct {
	Registry        *sessions.SessionRegistry
	WebhookStore    *webhook.WebhookConfigStore
	Dispatcher      *dispatcher.Dispatcher
	MediaStore      *media.MediaStorage
	EventHub        *events.RealtimeHub
	ListenerManager *events.SessionListenerManager
	E2EEManager     *e2ee.ProtocolManager
	httpClient      *http.Client

	ticketsMu sync.Mutex
	tickets   map[string]time.Time

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
	if lManager == nil {
		lManager = events.NewSessionListenerManager(hub)
	}
	if mStore == nil {
		mStore = media.NewMediaStorage("")
	}
	if reg == nil {
		encStore := sessions.NewEncryptedSessionStore("")
		reg = sessions.NewSessionRegistry(encStore, nil)
	}

	return &Server{
		Registry:        reg,
		WebhookStore:    wsStore,
		Dispatcher:      disp,
		MediaStore:      mStore,
		EventHub:        hub,
		ListenerManager: lManager,
		E2EEManager:     e2ee.NewProtocolManager(),
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		tickets:         make(map[string]time.Time),
	}
}

// IssueWebSocketTicket emite un token de un solo uso válido por 30 segundos.
func (s *Server) IssueWebSocketTicket() string {
	s.ticketsMu.Lock()
	defer s.ticketsMu.Unlock()

	now := time.Now()
	for t, exp := range s.tickets {
		if now.After(exp) {
			delete(s.tickets, t)
		}
	}

	b := make([]byte, 32)
	_, _ = rand.Read(b)
	ticket := base64.RawURLEncoding.EncodeToString(b)
	s.tickets[ticket] = now.Add(WebSocketTicketTTLSecs * time.Second)
	return ticket
}

// ConsumeWebSocketTicket valida y destruye el ticket si sigue vigente.
func (s *Server) ConsumeWebSocketTicket(ticket string) bool {
	if ticket == "" {
		return false
	}
	s.ticketsMu.Lock()
	defer s.ticketsMu.Unlock()

	exp, found := s.tickets[ticket]
	if !found {
		return false
	}
	delete(s.tickets, ticket)
	return time.Now().Before(exp)
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

// ReadinessReport evalúa el estado de salud y configuración del microservicio.
func (s *Server) ReadinessReport() map[string]any {
	reqAuth := os.Getenv("FBCHAT_API_AUTH_REQUIRED") != "0"
	userSet := os.Getenv("FBCHAT_API_USERNAME") != ""
	passSet := os.Getenv("FBCHAT_API_PASSWORD") != ""
	authConfigured := userSet && passSet
	authReady := authConfigured || !reqAuth

	status := "ready"
	if !authReady {
		status = "not_ready"
	}

	return map[string]any{
		"status":  status,
		"service": ServiceName,
		"version": Version,
		"checks": map[string]any{
			"api_auth": map[string]any{
				"ready":      authReady,
				"required":   reqAuth,
				"configured": authConfigured,
			},
			"e2ee_bridge": map[string]any{
				"ready":      true,
				"required":   false,
				"configured": true,
			},
			"portal": map[string]any{
				"ready": true,
			},
		},
	}
}
