package e2ee

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrProtocolUnavailable = errors.New("el protocolo E2EE no está disponible en esta compilación")

// ProtocolManager exposes the E2EE facade without claiming a backend exists.
type ProtocolManager struct {
	mu          sync.RWMutex
	initialized bool
	backend     RPCCaller
	actions     *BridgeActions
}

func NewProtocolManager() *ProtocolManager {
	mgr := &ProtocolManager{}
	mgr.actions = NewBridgeActions(mgr)
	return mgr
}

func (p *ProtocolManager) Actions() *BridgeActions { return p.actions }

// SetBackend asigna el proveedor o backend real de ejecución E2EE (como whatsmeow o puente RPC).
func (p *ProtocolManager) SetBackend(backend RPCCaller) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.backend = backend
	p.initialized = (backend != nil)
}

// Available reports whether a real protocol backend has been configured.
func (p *ProtocolManager) Available() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.backend != nil && p.initialized
}

func (p *ProtocolManager) Call(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	p.mu.RLock()
	backend := p.backend
	p.mu.RUnlock()

	if backend != nil {
		return backend.Call(ctx, method, params)
	}
	return nil, fmt.Errorf("%w: %s", ErrProtocolUnavailable, method)
}

func (p *ProtocolManager) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.backend = nil
	p.initialized = false
	return nil
}
