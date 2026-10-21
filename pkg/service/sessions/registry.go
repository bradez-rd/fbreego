package sessions

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/session"
	"github.com/bradez-rd/fbreego/pkg/core/types"
)

// SessionRegistry gestiona concurrentemente en memoria el ciclo de vida y estado de las sesiones.
type SessionRegistry struct {
	mu         sync.RWMutex
	sessions   map[string]*SessionRecord
	storage    *EncryptedSessionStore
	httpClient *http.Client
}

// NewSessionRegistry inicializa un nuevo registro de sesiones, cargando las sesiones guardadas si el store está presente.
func NewSessionRegistry(store *EncryptedSessionStore, httpClient *http.Client) *SessionRegistry {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	reg := &SessionRegistry{
		sessions:   make(map[string]*SessionRecord),
		storage:    store,
		httpClient: httpClient,
	}

	if store != nil {
		loaded, err := store.LoadAll()
		if err == nil {
			for _, rec := range loaded {
				reg.sessions[rec.SessionID] = rec
			}
		}
	}

	return reg
}

// SetHTTPClient asigna un cliente HTTP personalizado (p.ej. con interceptor de tráfico).
func (r *SessionRegistry) SetHTTPClient(client *http.Client) {
	if client != nil {
		r.mu.Lock()
		r.httpClient = client
		r.mu.Unlock()
	}
}

// HTTPClient obtiene el cliente HTTP actual configurado en el registry.
func (r *SessionRegistry) HTTPClient() *http.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.httpClient
}

// generateUUIDv4 crea un identificador único para cada sesión registrada.
func generateUUIDv4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Create registra una nueva sesión a partir de una lista de cookies, validándola inmediatamente contra Facebook.
func (r *SessionRegistry) Create(ctx context.Context, input types.SessionCreateInput) (*SessionRecord, error) {
	if len(input.Cookies) < 2 {
		return nil, fmt.Errorf("se requieren al menos 2 cookies para inicializar la sesión")
	}

	tenantID := input.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	label := input.Label
	if label == "" {
		label = "Facebook account"
	}

	var cookieHeader string
	for _, c := range input.Cookies {
		cookieHeader += c.Name + "=" + c.Value + "; "
	}

	// Validar contra Facebook
	providerData, err := session.ExtractSession(ctx, r.httpClient, cookieHeader, nil)
	if err != nil {
		return nil, fmt.Errorf("error al validar sesión con Facebook: %w", err)
	}

	sessionID := generateUUIDv4()
	now := UTCNow()

	record := &SessionRecord{
		SessionID:    sessionID,
		Label:        label,
		TenantID:     tenantID,
		Cookies:      input.Cookies,
		FacebookID:   providerData.FacebookID,
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		ProviderData: providerData,
	}

	r.mu.Lock()
	r.sessions[sessionID] = record
	r.mu.Unlock()

	if r.storage != nil {
		_ = r.storage.Save(record)
	}

	return record, nil
}

// ListPublic devuelve la lista de sesiones sanitizadas, filtrando opcionalmente por inquilino.
func (r *SessionRegistry) ListPublic(tenantID string, details bool) []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []map[string]any
	for _, rec := range r.sessions {
		if tenantID != "" && rec.TenantID != tenantID {
			continue
		}
		if details {
			results = append(results, rec.Detailed())
		} else {
			results = append(results, rec.Public())
		}
	}
	if results == nil {
		results = []map[string]any{}
	}
	return results
}

// Get obtiene el registro completo de una sesión por su ID.
func (r *SessionRegistry) Get(sessionID string) (*SessionRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rec, exists := r.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("sesión no encontrada: %s", sessionID)
	}
	return rec, nil
}

// Validate vuelve a extraer los tokens de la sesión contra Facebook y actualiza el registro.
func (r *SessionRegistry) Validate(ctx context.Context, sessionID string) (*SessionRecord, error) {
	rec, err := r.Get(sessionID)
	if err != nil {
		return nil, err
	}

	cookieHeader := rec.BuildCookieHeader()
	providerData, err := session.ExtractSession(ctx, r.httpClient, cookieHeader, nil)

	r.mu.Lock()
	defer r.mu.Unlock()

	now := UTCNow()
	rec.LastActivity = now

	if err != nil {
		rec.Status = "error"
		rec.LastError = err.Error()
		if r.storage != nil {
			_ = r.storage.Save(rec)
		}
		return nil, fmt.Errorf("la sesión es inválida o ha caducado: %w", err)
	}

	rec.Status = "active"
	rec.LastError = ""
	rec.FacebookID = providerData.FacebookID
	rec.ProviderData = providerData

	if r.storage != nil {
		_ = r.storage.Save(rec)
	}

	return rec, nil
}

// Delete remueve una sesión de memoria y del disco.
func (r *SessionRegistry) Delete(sessionID string) error {
	r.mu.Lock()
	rec, exists := r.sessions[sessionID]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("sesión no encontrada: %s", sessionID)
	}
	delete(r.sessions, sessionID)
	r.mu.Unlock()

	if r.storage != nil {
		_ = r.storage.Delete(rec.TenantID, sessionID)
	}
	return nil
}

// RecordActivity actualiza la marca temporal de última actividad de la sesión.
func (r *SessionRegistry) RecordActivity(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, exists := r.sessions[sessionID]; exists {
		rec.LastActivity = UTCNow()
		if r.storage != nil {
			_ = r.storage.Save(rec)
		}
	}
}

// RecordError anota un error operativo y actualiza el estado de la sesión.
func (r *SessionRegistry) RecordError(sessionID string, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, exists := r.sessions[sessionID]; exists {
		rec.Status = "error"
		rec.LastError = errMsg
		rec.LastActivity = UTCNow()
		if r.storage != nil {
			_ = r.storage.Save(rec)
		}
	}
}

// Count devuelve la cantidad total de sesiones en memoria.
func (r *SessionRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}

// Close libera los recursos asociados al registro.
func (r *SessionRegistry) Close() {
	// Limpieza explícita si se requiere
}
