package events

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// EventSubscriber representa un cliente WebSocket conectado recibiendo eventos en tiempo real.
type EventSubscriber struct {
	ws        *websocket.Conn
	sendChan  chan []byte
	sessionID string
}

// EventDispatcher is implemented by asynchronous sinks such as the webhook dispatcher.
type EventDispatcher interface {
	Enqueue(eventName string, payload map[string]any) bool
}

// RealtimeHub coordina la distribución Pub/Sub de eventos a clientes WebSocket conectados.
type RealtimeHub struct {
	mu          sync.RWMutex
	subscribers map[*EventSubscriber]bool
	totalEvents int64
	dispatcher  EventDispatcher
}

// SetDispatcher connects the realtime event source to an asynchronous external sink.
func (h *RealtimeHub) SetDispatcher(dispatcher EventDispatcher) {
	h.mu.Lock()
	h.dispatcher = dispatcher
	h.mu.Unlock()
}

// NewRealtimeHub inicializa un nuevo hub de distribución de eventos.
func NewRealtimeHub() *RealtimeHub {
	return &RealtimeHub{
		subscribers: make(map[*EventSubscriber]bool),
	}
}

// Register agrega un nuevo cliente WebSocket al hub.
func (h *RealtimeHub) Register(ws *websocket.Conn, sessionID string) *EventSubscriber {
	sub := &EventSubscriber{
		ws:        ws,
		sendChan:  make(chan []byte, 128),
		sessionID: sessionID,
	}

	h.mu.Lock()
	h.subscribers[sub] = true
	h.mu.Unlock()

	return sub
}

// Unregister retira un cliente WebSocket y cierra su canal de envío.
func (h *RealtimeHub) Unregister(sub *EventSubscriber) {
	h.mu.Lock()
	if _, exists := h.subscribers[sub]; exists {
		delete(h.subscribers, sub)
		close(sub.sendChan)
	}
	h.mu.Unlock()
}

// ClientCount devuelve el número actual de clientes WebSocket conectados.
func (h *RealtimeHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}

// Metrics devuelve estadísticas de clientes y volumen de eventos.
func (h *RealtimeHub) Metrics() map[string]any {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return map[string]any{
		"connectedClients": len(h.subscribers),
		"totalEvents":      h.totalEvents,
	}
}

// Broadcast distribuye un evento JSON a todos los clientes suscritos (o filtrados por sessionID si corresponde).
func (h *RealtimeHub) Broadcast(event map[string]any) {
	raw, err := json.Marshal(event)
	if err != nil {
		return
	}

	targetSession, _ := event["sessionId"].(string)

	h.mu.Lock()
	h.totalEvents++
	h.mu.Unlock()

	h.mu.RLock()
	defer h.mu.RUnlock()

	for sub := range h.subscribers {
		if sub.sessionID != "" && targetSession != "" && sub.sessionID != targetSession {
			continue
		}
		select {
		case sub.sendChan <- raw:
		default:
			// Descartar si el buffer del cliente está saturado
		}
	}
}

// Publish sends an event to local WebSocket subscribers and the configured webhook sink.
func (h *RealtimeHub) Publish(event map[string]any) {
	h.Broadcast(event)
	eventName, _ := event["event"].(string)
	if eventName == "" {
		return
	}
	h.mu.RLock()
	dispatcher := h.dispatcher
	h.mu.RUnlock()
	if dispatcher != nil {
		dispatcher.Enqueue(eventName, event)
	}
}

// EmitSessionStatus emite una actualización de estado de una sesión.
func (h *RealtimeHub) EmitSessionStatus(sessionID, status, errorMessage string) {
	evt := map[string]any{
		"event":     "session_status",
		"type":      "session_status",
		"sessionId": sessionID,
		"status":    status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if errorMessage != "" {
		evt["error"] = errorMessage
	}
	h.Publish(evt)
}

// EmitMessageReceived emite un mensaje entrante decodificado.
func (h *RealtimeHub) EmitMessageReceived(sessionID string, msg any) {
	evt := map[string]any{
		"event":     "message_received",
		"sessionId": sessionID,
		"message":   msg,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	h.Publish(evt)
}

// PumpWrites bombea los mensajes del canal hacia la conexión WebSocket del suscriptor.
func (sub *EventSubscriber) PumpWrites(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-sub.sendChan:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := sub.ws.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
