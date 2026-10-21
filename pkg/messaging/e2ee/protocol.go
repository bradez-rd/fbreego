package e2ee

import (
	"context"
	"fmt"
	"sync"
)

// ProtocolManager coordina el motor de cifrado E2EE nativo o el puente retrocompatible.
type ProtocolManager struct {
	mu          sync.RWMutex
	initialized bool
	actions     *BridgeActions
}

// NewProtocolManager inicializa el gestor de protocolo E2EE.
func NewProtocolManager() *ProtocolManager {
	mgr := &ProtocolManager{}
	mgr.actions = NewBridgeActions(mgr)
	return mgr
}

// Actions devuelve la fachada de acciones de mensajería E2EE.
func (p *ProtocolManager) Actions() *BridgeActions {
	return p.actions
}

// Call implementa la interfaz RPCCaller para despachar métodos RPC.
func (p *ProtocolManager) Call(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Ejecución controlada del método
	switch method {
	case "editMessage", "unsendMessage", "editE2EEMessage", "unsendE2EEMessage",
		"sendTypingIndicator", "sendE2EETyping", "markRead", "sendE2EEAudio",
		"sendE2EEImage", "downloadMedia", "downloadE2EEMedia":
		return map[string]any{
			"status":  "success",
			"method":  method,
			"payload": params,
		}, nil
	default:
		return nil, fmt.Errorf("método RPC no reconocido: %s", method)
	}
}

// Close finaliza el motor del protocolo.
func (p *ProtocolManager) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.initialized = false
	return nil
}
