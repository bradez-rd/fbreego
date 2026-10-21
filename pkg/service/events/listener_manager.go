package events

import (
	"context"
	"fmt"
	"sync"
)

// SessionListenerManager coordina la ejecución en background de los listeners MQTT de cada sesión activa.
type SessionListenerManager struct {
	mu        sync.RWMutex
	listeners map[string]context.CancelFunc
	statuses  map[string]string
	hub       *RealtimeHub
}

// NewSessionListenerManager crea un nuevo gestor de listeners de sesión.
func NewSessionListenerManager(hub *RealtimeHub) *SessionListenerManager {
	return &SessionListenerManager{
		listeners: make(map[string]context.CancelFunc),
		statuses:  make(map[string]string),
		hub:       hub,
	}
}

// IsListening verifica si un listener está actualmente en ejecución para una sesión.
func (m *SessionListenerManager) IsListening(sessionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.listeners[sessionID]
	return exists
}

// GetStatus obtiene el estado de escucha de la sesión ("listening", "stopped", "error").
func (m *SessionListenerManager) GetStatus(sessionID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status, exists := m.statuses[sessionID]
	if !exists {
		return "stopped"
	}
	return status
}

// StartListening inicia la ejecución en segundo plano de la función de escucha para la sesión.
func (m *SessionListenerManager) StartListening(ctx context.Context, sessionID string, worker func(ctx context.Context)) error {
	m.mu.Lock()
	if _, exists := m.listeners[sessionID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("el listener para la sesión %s ya está en ejecución", sessionID)
	}

	listenerCtx, cancel := context.WithCancel(ctx)
	m.listeners[sessionID] = cancel
	m.statuses[sessionID] = "listening"
	m.mu.Unlock()

	if m.hub != nil {
		m.hub.EmitSessionStatus(sessionID, "listening", "")
	}

	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.listeners, sessionID)
			m.statuses[sessionID] = "stopped"
			m.mu.Unlock()
			if m.hub != nil {
				m.hub.EmitSessionStatus(sessionID, "stopped", "")
			}
		}()

		worker(listenerCtx)
	}()

	return nil
}

// StopListening detiene la tarea de escucha de una sesión específica.
func (m *SessionListenerManager) StopListening(sessionID string) bool {
	m.mu.Lock()
	cancel, exists := m.listeners[sessionID]
	if exists {
		delete(m.listeners, sessionID)
		m.statuses[sessionID] = "stopped"
		cancel()
	}
	m.mu.Unlock()

	if exists && m.hub != nil {
		m.hub.EmitSessionStatus(sessionID, "stopped", "")
	}
	return exists
}

// StopAll detiene todos los listeners de sesión en ejecución.
func (m *SessionListenerManager) StopAll() {
	m.mu.Lock()
	for id, cancel := range m.listeners {
		cancel()
		delete(m.listeners, id)
		m.statuses[id] = "stopped"
		if m.hub != nil {
			m.hub.EmitSessionStatus(id, "stopped", "")
		}
	}
	m.mu.Unlock()
}
