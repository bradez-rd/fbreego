package sessions

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"strings"
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
	initErr    error
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
		if err != nil {
			reg.initErr = err
		} else {
			for _, rec := range loaded {
				if rec.ValidatedAt == "" {
					rec.ValidatedAt = rec.CreatedAt
				}
				if rec.LastActivityAt == "" {
					rec.LastActivityAt = rec.LastActivity
				}
				if rec.LastActivityAt == "" {
					rec.LastActivityAt = rec.CreatedAt
				}
				if rec.LastActivity == "" {
					rec.LastActivity = rec.LastActivityAt
				}
				reg.sessions[rec.SessionID] = rec
			}
		}
	}

	return reg
}

// Ready reports whether persisted session state was loaded without data loss.
func (r *SessionRegistry) Ready() error {
	if r == nil {
		return fmt.Errorf("registro de sesiones no configurado")
	}
	return r.initErr
}

// HasEncryptedStorage reporta si el registro está respaldado por almacenamiento cifrado persistente.
func (r *SessionRegistry) HasEncryptedStorage() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.storage != nil
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
	if err := r.Ready(); err != nil {
		return nil, err
	}
	if len(input.Cookies) < 2 {
		return nil, fmt.Errorf("se requieren al menos 2 cookies para inicializar la sesión")
	}

	seenCookies := make(map[string]bool)
	hasCUser := false
	hasXS := false
	nowUnix := float64(time.Now().Unix())

	for _, c := range input.Cookies {
		trimmedName := strings.TrimSpace(c.Name)
		if trimmedName == "" {
			return nil, fmt.Errorf("nombre de cookie no puede estar vacío")
		}
		if seenCookies[trimmedName] {
			return nil, fmt.Errorf("cookie duplicada en la entrada: %s", trimmedName)
		}
		seenCookies[trimmedName] = true

		if trimmedName == "c_user" {
			hasCUser = true
		}
		if trimmedName == "xs" {
			hasXS = true
		}
		if c.ExpirationDate > 0 && c.ExpirationDate < nowUnix {
			return nil, fmt.Errorf("la cookie '%s' ya ha expirado", trimmedName)
		}
	}
	if !hasCUser || !hasXS {
		return nil, fmt.Errorf("se requieren obligatoriamente las cookies 'c_user' y 'xs' para una sesión válida")
	}

	tenantID := input.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	if err := ValidateTenantID(tenantID); err != nil {
		return nil, err
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

	now := UTCNow()

	// Deduplicación por cuenta dentro del tenant (F-06): si ya existe, actualizarla para no crear sesiones concurrentes huérfanas
	r.mu.Lock()
	for _, existing := range r.sessions {
		if existing.TenantID == tenantID && existing.FacebookID == providerData.FacebookID {
			existing.Cookies = input.Cookies
			existing.ProviderData = providerData
			existing.ValidatedAt = now
			existing.LastActivityAt = now
			existing.LastActivity = now
			existing.Status = StatusActive
			existing.LastError = ""
			if input.Label != "" {
				existing.Label = input.Label
			}
			r.mu.Unlock()
			if r.storage != nil {
				_ = r.storage.Save(existing)
			}
			return existing, nil
		}
	}
	r.mu.Unlock()

	sessionID := generateUUIDv4()

	record := &SessionRecord{
		SessionID:      sessionID,
		Label:          label,
		TenantID:       tenantID,
		Cookies:        input.Cookies,
		FacebookID:     providerData.FacebookID,
		Status:         StatusActive,
		CreatedAt:      now,
		ValidatedAt:    now,
		LastActivityAt: now,
		LastActivity:   now,
		ProviderData:   providerData,
	}

	r.mu.Lock()
	r.sessions[sessionID] = record
	r.mu.Unlock()

	if r.storage != nil {
		if err := r.storage.Save(record); err != nil {
			r.mu.Lock()
			delete(r.sessions, sessionID)
			r.mu.Unlock()
			return nil, fmt.Errorf("error al persistir sesión: %w", err)
		}
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
	if err := r.Ready(); err != nil {
		return nil, err
	}
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
	rec.LastActivityAt = now

	if err != nil {
		rec.Status = "error"
		rec.LastError = err.Error()
		if r.storage != nil {
			if saveErr := r.storage.Save(rec); saveErr != nil {
				return nil, fmt.Errorf("error al persistir estado inválido: %w", saveErr)
			}
		}
		return nil, fmt.Errorf("la sesión es inválida o ha caducado: %w", err)
	}

	rec.Status = "active"
	rec.LastError = ""
	rec.FacebookID = providerData.FacebookID
	rec.ProviderData = providerData
	now = UTCNow()
	rec.ValidatedAt = now
	rec.LastActivityAt = now
	rec.LastActivity = now

	if r.storage != nil {
		if err := r.storage.Save(rec); err != nil {
			return nil, fmt.Errorf("error al persistir sesión validada: %w", err)
		}
	}

	return rec, nil
}

// GetForTenant obtiene una sesión validando que pertenezca al inquilino especificado (si se especifica tenantID).
func (r *SessionRegistry) GetForTenant(sessionID, tenantID string) (*SessionRecord, error) {
	rec, err := r.Get(sessionID)
	if err != nil {
		return nil, err
	}
	if tenantID != "" && rec.TenantID != tenantID {
		return nil, fmt.Errorf("acceso denegado: la sesión pertenece a otro inquilino")
	}
	return rec, nil
}

// Delete remueve una sesión de memoria y del disco marcando su estado como stopped.
func (r *SessionRegistry) Delete(sessionID string) error {
	return r.DeleteForTenant(sessionID, "")
}

// DeleteForTenant remueve una sesión validando la pertenencia de inquilino.
func (r *SessionRegistry) DeleteForTenant(sessionID, tenantID string) error {
	if err := r.Ready(); err != nil {
		return err
	}
	r.mu.Lock()
	rec, exists := r.sessions[sessionID]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("sesión no encontrada: %s", sessionID)
	}
	if tenantID != "" && rec.TenantID != tenantID {
		r.mu.Unlock()
		return fmt.Errorf("acceso denegado: la sesión pertenece a otro inquilino")
	}
	rec.Status = StatusStopped
	if r.storage != nil {
		if err := r.storage.Delete(rec.TenantID, sessionID); err != nil && !os.IsNotExist(err) {
			r.mu.Unlock()
			return fmt.Errorf("error al eliminar sesión persistida: %w", err)
		}
	}
	delete(r.sessions, sessionID)
	r.mu.Unlock()
	return nil
}

// RegisterRecord registra directamente una sesión en memoria (útil para inicializaciones y pruebas).
func (r *SessionRegistry) RegisterRecord(rec *SessionRecord) {
	if rec == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[rec.SessionID] = rec
}

// RecordActivity actualiza la marca temporal de última actividad de la sesión.
func (r *SessionRegistry) RecordActivity(sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, exists := r.sessions[sessionID]; exists {
		now := UTCNow()
		rec.LastActivity = now
		rec.LastActivityAt = now
		if r.storage != nil {
			if err := r.storage.Save(rec); err != nil {
				return fmt.Errorf("error al persistir actividad de sesión: %w", err)
			}
		}
	}
	return nil
}

// RecordError anota un error operativo y actualiza el estado de la sesión.
func (r *SessionRegistry) RecordError(sessionID string, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, exists := r.sessions[sessionID]; exists {
		rec.Status = "error"
		rec.LastError = errMsg
		now := UTCNow()
		rec.LastActivity = now
		rec.LastActivityAt = now
		if r.storage != nil {
			if err := r.storage.Save(rec); err != nil {
				return fmt.Errorf("error al persistir estado de error de sesión: %w", err)
			}
		}
	}
	return nil
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
