package webhook

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	coreconfig "github.com/bradez-rd/fbreego/pkg/core/config"
	"github.com/bradez-rd/fbreego/pkg/core/permissions"
)

const webhookOutboxFilename = "webhook_outbox.enc"

// WebhookJob representa una entrega durable pendiente hacia el consumidor configurado.
type WebhookJob struct {
	ID            string         `json:"id"`
	EventName     string         `json:"eventName"`
	Payload       map[string]any `json:"payload"`
	CreatedAt     string         `json:"createdAt"`
	Attempts      int            `json:"attempts"`
	NextAttemptAt string         `json:"nextAttemptAt"`
	LastError     string         `json:"lastError,omitempty"`
}

// WebhookOutboxStore conserva cifradas las entregas pendientes para sobrevivir reinicios.
type WebhookOutboxStore struct {
	mu       sync.RWMutex
	stateDir string
	key      []byte
	jobs     map[string]WebhookJob
	initErr  error
}

func NewWebhookOutboxStore(stateDir string) *WebhookOutboxStore {
	if stateDir == "" {
		stateDir = coreconfig.DataDir("FBREEGO_SESSIONS_DIR", ".fbreego_sessions")
	}
	store := &WebhookOutboxStore{
		stateDir: stateDir,
		jobs:     make(map[string]WebhookJob),
	}
	secret := coreconfig.Value("FBREEGO_SESSION_SECRET")
	if len(secret) < 32 {
		store.initErr = fmt.Errorf("FBREEGO_SESSION_SECRET debe contener al menos 32 caracteres")
		return store
	}
	hash := sha256.Sum256([]byte(secret))
	store.key = hash[:]
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		store.initErr = fmt.Errorf("no se pudo crear el directorio del outbox: %w", err)
		return store
	}
	if err := permissions.SetPrivateFilePermissions(stateDir); err != nil {
		store.initErr = fmt.Errorf("no se pudo proteger el directorio del outbox: %w", err)
		return store
	}
	if err := store.load(); err != nil {
		store.initErr = err
	}
	return store
}

func (s *WebhookOutboxStore) Ready() error {
	if s == nil {
		return fmt.Errorf("outbox de webhook no configurado")
	}
	return s.initErr
}

func (s *WebhookOutboxStore) path() string {
	return filepath.Join(s.stateDir, webhookOutboxFilename)
}

func (s *WebhookOutboxStore) load() error {
	data, err := os.ReadFile(s.path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("no se pudo leer el outbox de webhook: %w", err)
	}
	plain, err := decryptOutbox(data, s.key)
	if err != nil {
		return fmt.Errorf("no se pudo descifrar el outbox de webhook: %w", err)
	}
	var jobs []WebhookJob
	if err := json.Unmarshal(plain, &jobs); err != nil {
		return fmt.Errorf("outbox de webhook inválido: %w", err)
	}
	for _, job := range jobs {
		if job.ID == "" || job.EventName == "" {
			continue
		}
		s.jobs[job.ID] = job
	}
	return nil
}

func decryptOutbox(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, fmt.Errorf("datos cifrados incompletos")
	}
	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
}

func encryptOutbox(plain, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
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
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func (s *WebhookOutboxStore) persistLocked() error {
	jobs := make([]WebhookJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt < jobs[j].CreatedAt })
	raw, err := json.Marshal(jobs)
	if err != nil {
		return err
	}
	encrypted, err := encryptOutbox(raw, s.key)
	if err != nil {
		return err
	}
	temp := s.path() + ".tmp"
	if err := os.WriteFile(temp, encrypted, 0600); err != nil {
		return err
	}
	if err := permissions.SetPrivateFilePermissions(temp); err != nil {
		_ = os.Remove(temp)
		return err
	}
	if err := os.Rename(temp, s.path()); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return permissions.SetPrivateFilePermissions(s.path())
}

func newWebhookJobID() (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (s *WebhookOutboxStore) Add(eventName string, payload map[string]any) (WebhookJob, error) {
	if err := s.Ready(); err != nil {
		return WebhookJob{}, err
	}
	id, err := newWebhookJobID()
	if err != nil {
		return WebhookJob{}, err
	}
	now := time.Now().UTC()
	job := WebhookJob{
		ID:            id,
		EventName:     eventName,
		Payload:       payload,
		CreatedAt:     now.Format(time.RFC3339Nano),
		NextAttemptAt: now.Format(time.RFC3339Nano),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[id] = job
	if err := s.persistLocked(); err != nil {
		delete(s.jobs, id)
		return WebhookJob{}, err
	}
	return job, nil
}

func (s *WebhookOutboxStore) Due(now time.Time) []WebhookJob {
	if s == nil || s.Ready() != nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]WebhookJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		next, err := time.Parse(time.RFC3339Nano, job.NextAttemptAt)
		if err == nil && next.After(now) {
			continue
		}
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt < jobs[j].CreatedAt })
	return jobs
}

func (s *WebhookOutboxStore) Delete(id string) error {
	if err := s.Ready(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[id]; !exists {
		return nil
	}
	job := s.jobs[id]
	delete(s.jobs, id)
	if err := s.persistLocked(); err != nil {
		s.jobs[id] = job
		return err
	}
	return nil
}

func (s *WebhookOutboxStore) MarkFailed(id string, deliveryError error, nextAttempt time.Time) error {
	if err := s.Ready(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	job, exists := s.jobs[id]
	if !exists {
		return nil
	}
	original := job
	job.Attempts++
	job.NextAttemptAt = nextAttempt.UTC().Format(time.RFC3339Nano)
	if deliveryError != nil {
		job.LastError = deliveryError.Error()
	}
	s.jobs[id] = job
	if err := s.persistLocked(); err != nil {
		s.jobs[id] = original
		return err
	}
	return nil
}

func (s *WebhookOutboxStore) Count() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.jobs)
}
