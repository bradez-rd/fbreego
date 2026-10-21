package sessions

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/httputil"
	"github.com/bradez-rd/fbreego/pkg/core/types"
)

const (
	StatusActive      = "active"
	StatusError       = "error"
	StatusUnvalidated = "unvalidated"
	StatusStopped     = "stopped"
	StatusBlocked     = "blocked"
)

// SessionRecord representa el estado y credenciales completas de una sesión administrada.
type SessionRecord struct {
	SessionID      string              `json:"sessionId"`
	Label          string              `json:"label"`
	TenantID       string              `json:"tenantId"`
	Cookies        []types.CookieInput `json:"cookies,omitempty"`
	FacebookID     string              `json:"facebookId"`
	Status         string              `json:"status"` // "active", "error", "unvalidated", "stopped", "blocked"
	CreatedAt      string              `json:"createdAt"`
	ValidatedAt    string              `json:"validatedAt"`
	LastActivityAt string              `json:"lastActivityAt"`
	LastActivity   string              `json:"lastActivity,omitempty"`
	LastError      string              `json:"lastError,omitempty"`
	ProviderData   *types.DataFB       `json:"providerData,omitempty"`

	// Control de concurrencia y límites por cuenta (F-04)
	execMu       sync.Mutex
	lastOutbound time.Time
}

// CanExecute comprueba si la sesión está en un estado que permite emitir peticiones a Facebook.
func (r *SessionRecord) CanExecute() error {
	if r == nil {
		return fmt.Errorf("registro de sesión no válido")
	}
	switch r.Status {
	case StatusBlocked:
		return fmt.Errorf("operación denegada: la sesión se encuentra bloqueada por restricciones de Facebook (circuito abierto)")
	case StatusStopped:
		return fmt.Errorf("operación denegada: la sesión se encuentra detenida")
	}
	return nil
}

// AcquireExecutionSlot serializa las mutaciones salientes por cuenta y aplica un espaciado mínimo
// entre solicitudes consecutivas para mitigar detección por ráfagas concurrentes (F-04).
func (r *SessionRecord) AcquireExecutionSlot(ctx context.Context) error {
	if err := r.CanExecute(); err != nil {
		return err
	}

	// Esperar turno de ejecución serializado
	r.execMu.Lock()

	// Re-verificar estado tras adquirir bloqueo
	if err := r.CanExecute(); err != nil {
		r.execMu.Unlock()
		return err
	}

	// Espaciado mínimo de 300ms entre solicitudes a la misma cuenta
	const minInterval = 300 * time.Millisecond
	elapsed := time.Since(r.lastOutbound)
	if elapsed < minInterval {
		delay := minInterval - elapsed
		select {
		case <-ctx.Done():
			r.execMu.Unlock()
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	r.lastOutbound = time.Now()
	return nil
}

// ReleaseExecutionSlot libera el turno de ejecución serializado de la cuenta.
func (r *SessionRecord) ReleaseExecutionSlot() {
	r.execMu.Unlock()
}

// HandleHTTPResult clasifica el resultado de una llamada HTTP contra Facebook y actualiza
// el estado del registro (circuito abierto ante bloqueo o credenciales revocadas).
func (r *SessionRecord) HandleHTTPResult(err error) {
	if r == nil || err == nil {
		return
	}
	now := UTCNow()
	r.LastActivityAt = now
	r.LastActivity = now

	if errors.Is(err, httputil.ErrAccountRestricted) {
		r.Status = StatusBlocked
		r.LastError = "Cuenta restringida o bloqueada por Facebook (HTTP 403 / checkpoint)"
	} else if errors.Is(err, httputil.ErrSessionExpired) {
		r.Status = StatusError
		r.LastError = "Sesión expirada o credenciales revocadas (HTTP 401)"
	} else if errors.Is(err, httputil.ErrRateLimited) {
		r.LastError = "Límite de tasa excedido temporalmente (HTTP 429)"
	} else {
		r.LastError = "Error en comunicación con proveedor"
	}
}

// BuildCookieHeader concatena las cookies del registro en una cabecera HTTP estándar.
func (r *SessionRecord) BuildCookieHeader() string {
	if r == nil || len(r.Cookies) == 0 {
		return ""
	}
	var res string
	for _, c := range r.Cookies {
		if c.Name != "" && c.Value != "" {
			res += c.Name + "=" + c.Value + "; "
		}
	}
	return res
}

// Public devuelve un mapa seguro para el consumidor externo sin exponer cookies ni tokens.
func (r *SessionRecord) Public() map[string]any {
	lastAct := r.LastActivityAt
	if lastAct == "" {
		lastAct = r.LastActivity
	}
	valAt := r.ValidatedAt
	if valAt == "" {
		valAt = r.CreatedAt
	}
	return map[string]any{
		"sessionId":      r.SessionID,
		"label":          r.Label,
		"tenantId":       r.TenantID,
		"status":         r.Status,
		"createdAt":      r.CreatedAt,
		"validatedAt":    valAt,
		"lastActivityAt": lastAct,
		"lastActivity":   lastAct,
		"lastError":      r.LastError,
		"facebookId":     r.FacebookID,
	}
}

// Detailed devuelve información ampliada para diagnóstico administrativo sin comprometer claves privadas.
func (r *SessionRecord) Detailed() map[string]any {
	pub := r.Public()
	pub["cookieCount"] = len(r.Cookies)
	hasData := r.ProviderData != nil
	pub["hasProviderData"] = hasData
	if hasData {
		pub["clientRevision"] = r.ProviderData.ClientRevision
		pub["sessionID"] = r.ProviderData.SessionID
	}
	return pub
}

// ClearSecrets elimina las cookies y los datos de proveedor de la memoria del registro.
func (r *SessionRecord) ClearSecrets() {
	r.Cookies = nil
	r.ProviderData = nil
}

// UTCNow devuelve la fecha y hora actual en formato ISO 8601 UTC.
func UTCNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}
