package sessions

import (
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/types"
)

// SessionRecord representa el estado y credenciales completas de una sesión administrada.
type SessionRecord struct {
	SessionID      string              `json:"sessionId"`
	Label          string              `json:"label"`
	TenantID       string              `json:"tenantId"`
	Cookies        []types.CookieInput `json:"cookies,omitempty"`
	FacebookID     string              `json:"facebookId"`
	Status         string              `json:"status"` // "active", "error", "unvalidated", "stopped"
	CreatedAt      string              `json:"createdAt"`
	ValidatedAt    string              `json:"validatedAt"`
	LastActivityAt string              `json:"lastActivityAt"`
	LastActivity   string              `json:"lastActivity,omitempty"`
	LastError      string              `json:"lastError,omitempty"`
	ProviderData   *types.DataFB       `json:"providerData,omitempty"`
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
