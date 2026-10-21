package webhook

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/permissions"
)

var allowedEvents = map[string]bool{
	"message":        true,
	"session_status": true,
	"reaction":       true,
	"error":          true,
	"raw_event":      true,
}

// WebhookConfig representa la configuración persistida del webhook global.
type WebhookConfig struct {
	Enabled   bool     `json:"enabled"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Secret    string   `json:"secret,omitempty"`
	UpdatedAt string   `json:"updatedAt"`
}

// Public devuelve un mapa seguro sin exponer el secreto HMAC completo.
func (c *WebhookConfig) Public() map[string]any {
	return map[string]any{
		"enabled":   c.Enabled,
		"url":       c.URL,
		"events":    c.Events,
		"hasSecret": strings.TrimSpace(c.Secret) != "",
		"updatedAt": c.UpdatedAt,
	}
}

// WebhookConfigStore gestiona el almacenamiento cifrado en disco de la configuración de webhook.
type WebhookConfigStore struct {
	mu       sync.RWMutex
	stateDir string
	key      []byte
	cached   *WebhookConfig
}

// NewWebhookConfigStore crea una nueva instancia del almacén de configuración de webhook.
func NewWebhookConfigStore(stateDir string) *WebhookConfigStore {
	if stateDir == "" {
		stateDir = ".fbchat_sessions"
	}
	secret := os.Getenv("FBCHAT_SESSION_SECRET")
	if secret == "" {
		secret = "fbchat-default-development-secret-key-change-in-production"
	}
	hash := sha256.Sum256([]byte(secret))

	_ = os.MkdirAll(stateDir, 0700)
	_ = permissions.SetPrivateFilePermissions(stateDir)

	store := &WebhookConfigStore{
		stateDir: stateDir,
		key:      hash[:],
	}
	_ = store.load()
	return store
}

func (s *WebhookConfigStore) load() error {
	path := filepath.Join(s.stateDir, "webhook_config.enc")
	data, err := os.ReadFile(path)
	if err != nil {
		s.cached = &WebhookConfig{Enabled: false, Events: []string{}}
		return nil
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return fmt.Errorf("datos de webhook corruptos")
	}
	nonce, cipherText := data[:nonceSize], data[nonceSize:]
	plain, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return err
	}

	var conf WebhookConfig
	if err := json.Unmarshal(plain, &conf); err != nil {
		return err
	}
	s.cached = &conf
	return nil
}

// Get devuelve una copia segura de la configuración actual.
func (s *WebhookConfigStore) Get() WebhookConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cached == nil {
		return WebhookConfig{}
	}
	return *s.cached
}

// GetPublic devuelve los metadatos públicos de configuración.
func (s *WebhookConfigStore) GetPublic() map[string]any {
	cfg := s.Get()
	return cfg.Public()
}

// Update valida y persiste una nueva configuración de webhook.
func (s *WebhookConfigStore) Update(enabled bool, targetURL string, events []string, secret *string, clearSecret bool) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	targetURL = strings.TrimSpace(targetURL)
	if enabled && targetURL == "" {
		return nil, fmt.Errorf("la URL del webhook es obligatoria cuando está habilitado")
	}
	if targetURL != "" {
		u, err := url.Parse(targetURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, fmt.Errorf("la URL del webhook debe tener esquema http o https válido")
		}
	}

	var validEvents []string
	for _, e := range events {
		if allowedEvents[e] {
			validEvents = append(validEvents, e)
		}
	}

	currentSecret := ""
	if s.cached != nil {
		currentSecret = s.cached.Secret
	}
	if clearSecret {
		currentSecret = ""
	} else if secret != nil {
		currentSecret = *secret
	}

	newConfig := &WebhookConfig{
		Enabled:   enabled,
		URL:       targetURL,
		Events:    validEvents,
		Secret:    currentSecret,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	raw, err := json.Marshal(newConfig)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	encrypted := gcm.Seal(nonce, nonce, raw, nil)

	dest := filepath.Join(s.stateDir, "webhook_config.enc")
	temp := dest + ".tmp"
	if err := os.WriteFile(temp, encrypted, 0600); err != nil {
		return nil, err
	}
	_ = permissions.SetPrivateFilePermissions(temp)
	if err := os.Rename(temp, dest); err != nil {
		_ = os.Remove(temp)
		return nil, err
	}
	_ = permissions.SetPrivateFilePermissions(dest)

	s.cached = newConfig
	return newConfig.Public(), nil
}

// Clear borra la configuración persistida de webhook.
func (s *WebhookConfigStore) Clear() map[string]any {
	res, _ := s.Update(false, "", []string{}, nil, true)
	return res
}
