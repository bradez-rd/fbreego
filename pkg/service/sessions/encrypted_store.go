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
	"regexp"
	"strings"
	"sync"

	"github.com/bradez-rd/fbreego/pkg/core/permissions"
)

// EncryptedSessionStore persiste y recupera SessionRecord en disco cifrados con AES-256-GCM.
type EncryptedSessionStore struct {
	mu       sync.RWMutex
	stateDir string
	key      []byte
	initErr  error
}

var safeTenantID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

// ValidateTenantID rejects path traversal and values unsuitable as storage segments.
func ValidateTenantID(tenantID string) error {
	if tenantID == "" {
		return nil
	}
	if tenantID == "." || tenantID == ".." || !safeTenantID.MatchString(tenantID) {
		return fmt.Errorf("tenantId inválido: use 1-64 caracteres alfanuméricos, punto, guion o guion bajo")
	}
	return nil
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

	store := &EncryptedSessionStore{stateDir: stateDir}
	secret := strings.TrimSpace(os.Getenv("FBCHAT_SESSION_SECRET"))
	if len(secret) < 32 {
		store.initErr = fmt.Errorf("FBCHAT_SESSION_SECRET debe contener al menos 32 caracteres")
		return store
	}
	hash := sha256.Sum256([]byte(secret))
	store.key = hash[:]

	if err := os.MkdirAll(stateDir, 0700); err != nil {
		store.initErr = fmt.Errorf("no se pudo crear el directorio de sesiones: %w", err)
		return store
	}
	if err := permissions.SetPrivateFilePermissions(stateDir); err != nil {
		store.initErr = fmt.Errorf("no se pudo proteger el directorio de sesiones: %w", err)
	}

	return store
}

// Ready reports whether encrypted persistence was initialized safely.
func (s *EncryptedSessionStore) Ready() error {
	if s == nil {
		return fmt.Errorf("almacén de sesiones no configurado")
	}
	return s.initErr
}

func (s *EncryptedSessionStore) encrypt(data []byte) ([]byte, error) {
	if err := s.Ready(); err != nil {
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
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

func (s *EncryptedSessionStore) decrypt(data []byte) ([]byte, error) {
	if err := s.Ready(); err != nil {
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
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("datos cifrados corruptos o incompletos")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (s *EncryptedSessionStore) tenantDir(tenantID string) (string, error) {
	if err := s.Ready(); err != nil {
		return "", err
	}
	if tenantID == "" {
		tenantID = "default"
	}
	if err := ValidateTenantID(tenantID); err != nil {
		return "", err
	}
	dir := filepath.Join(s.stateDir, tenantID)
	root, err := filepath.Abs(s.stateDir)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	prefix := root + string(os.PathSeparator)
	if resolved != root && !strings.HasPrefix(resolved, prefix) {
		return "", fmt.Errorf("ruta tenant fuera del directorio de sesiones")
	}
	if err := os.MkdirAll(resolved, 0700); err != nil {
		return "", err
	}
	if err := permissions.SetPrivateFilePermissions(resolved); err != nil {
		return "", err
	}
	return resolved, nil
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

	dir, err := s.tenantDir(record.TenantID)
	if err != nil {
		return err
	}
	dest := filepath.Join(dir, record.SessionID+".enc")
	temp := dest + ".tmp"

	if err := os.WriteFile(temp, encrypted, 0600); err != nil {
		return fmt.Errorf("error al escribir archivo temporal cifrado: %w", err)
	}
	if err := permissions.SetPrivateFilePermissions(temp); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("error al proteger archivo temporal: %w", err)
	}

	if err := os.Rename(temp, dest); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("error al mover sesión cifrada a destino: %w", err)
	}
	if err := permissions.SetPrivateFilePermissions(dest); err != nil {
		return fmt.Errorf("error al proteger archivo de sesión: %w", err)
	}

	return nil
}

// Delete elimina una sesión cifrada del almacenamiento.
func (s *EncryptedSessionStore) Delete(tenantID, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir, err := s.tenantDir(tenantID)
	if err != nil {
		return err
	}
	target := filepath.Join(dir, sessionID+".enc")
	return os.Remove(target)
}

// LoadAll lee y descifra todas las sesiones encontradas en el directorio de estado.
func (s *EncryptedSessionStore) LoadAll() ([]*SessionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.Ready(); err != nil {
		return nil, err
	}

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
		if err := ValidateTenantID(tenantEntry.Name()); err != nil {
			return nil, fmt.Errorf("directorio tenant inválido %q: %w", tenantEntry.Name(), err)
		}
		tDir := filepath.Join(s.stateDir, tenantEntry.Name())
		files, err := os.ReadDir(tDir)
		if err != nil {
			return nil, fmt.Errorf("error al listar tenant %q: %w", tenantEntry.Name(), err)
		}
		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".enc" {
				continue
			}
			filePath := filepath.Join(tDir, f.Name())
			encData, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("error al leer sesión cifrada %q: %w", filePath, err)
			}
			decrypted, err := s.decrypt(encData)
			if err != nil {
				return nil, fmt.Errorf("error al descifrar sesión %q: %w", filePath, err)
			}
			var rec SessionRecord
			if err := json.Unmarshal(decrypted, &rec); err != nil {
				return nil, fmt.Errorf("registro de sesión inválido %q: %w", filePath, err)
			}
			results = append(results, &rec)
		}
	}

	return results, nil
}
