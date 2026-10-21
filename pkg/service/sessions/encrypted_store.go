package sessions

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/bradez-rd/fbreego/pkg/core/permissions"
)

// EncryptedSessionStore persiste y recupera SessionRecord en disco cifrados con AES-256-GCM.
type EncryptedSessionStore struct {
	mu       sync.RWMutex
	stateDir string
	key      []byte
}

// NewEncryptedSessionStore crea un nuevo almacén cifrado en la ruta indicada o en el directorio por defecto.
func NewEncryptedSessionStore(stateDir string) *EncryptedSessionStore {
	if stateDir == "" {
		envDir := os.Getenv("FBCHAT_SESSIONS_DIR")
		if envDir != "" {
			stateDir = envDir
		} else {
			stateDir = ".fbchat_sessions"
		}
	}

	secret := os.Getenv("FBCHAT_SESSION_SECRET")
	if secret == "" {
		secret = "fbchat-default-development-secret-key-change-in-production"
	}
	hash := sha256.Sum256([]byte(secret))

	_ = os.MkdirAll(stateDir, 0700)
	_ = permissions.SetPrivateFilePermissions(stateDir)

	return &EncryptedSessionStore{
		stateDir: stateDir,
		key:      hash[:],
	}
}

func (s *EncryptedSessionStore) encrypt(data []byte) ([]byte, error) {
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
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

func (s *EncryptedSessionStore) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("datos cifrados corruptos o incompletos")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (s *EncryptedSessionStore) tenantDir(tenantID string) string {
	if tenantID == "" {
		tenantID = "default"
	}
	dir := filepath.Join(s.stateDir, tenantID)
	_ = os.MkdirAll(dir, 0700)
	_ = permissions.SetPrivateFilePermissions(dir)
	return dir
}

// Save cifra y guarda el registro de sesión en disco.
func (s *EncryptedSessionStore) Save(record *SessionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("error al serializar registro de sesión: %w", err)
	}

	encrypted, err := s.encrypt(raw)
	if err != nil {
		return fmt.Errorf("error al cifrar registro de sesión: %w", err)
	}

	dir := s.tenantDir(record.TenantID)
	dest := filepath.Join(dir, record.SessionID+".enc")
	temp := dest + ".tmp"

	if err := os.WriteFile(temp, encrypted, 0600); err != nil {
		return fmt.Errorf("error al escribir archivo temporal cifrado: %w", err)
	}
	_ = permissions.SetPrivateFilePermissions(temp)

	if err := os.Rename(temp, dest); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("error al mover sesión cifrada a destino: %w", err)
	}
	_ = permissions.SetPrivateFilePermissions(dest)

	return nil
}

// Delete elimina una sesión cifrada del almacenamiento.
func (s *EncryptedSessionStore) Delete(tenantID, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.tenantDir(tenantID)
	target := filepath.Join(dir, sessionID+".enc")
	return os.Remove(target)
}

// LoadAll lee y descifra todas las sesiones encontradas en el directorio de estado.
func (s *EncryptedSessionStore) LoadAll() ([]*SessionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*SessionRecord
	tenants, err := os.ReadDir(s.stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, tenantEntry := range tenants {
		if !tenantEntry.IsDir() {
			continue
		}
		tDir := filepath.Join(s.stateDir, tenantEntry.Name())
		files, err := os.ReadDir(tDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".enc" {
				continue
			}
			filePath := filepath.Join(tDir, f.Name())
			encData, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			decrypted, err := s.decrypt(encData)
			if err != nil {
				continue
			}
			var rec SessionRecord
			if err := json.Unmarshal(decrypted, &rec); err == nil {
				results = append(results, &rec)
			}
		}
	}

	return results, nil
}
