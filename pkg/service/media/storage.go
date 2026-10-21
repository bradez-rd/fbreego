package media

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/permissions"
)

var filenameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// MediaRecord contiene los metadatos de un archivo multimedia almacenado localmente.
type MediaRecord struct {
	MediaID   string `json:"mediaId"`
	FileName  string `json:"fileName"`
	MimeType  string `json:"mimeType"`
	FileSize  int64  `json:"fileSize"`
	SHA256    string `json:"sha256"`
	CreatedAt string `json:"createdAt"`
}

// Public devuelve un mapa serializable seguro de los metadatos del medio.
func (r *MediaRecord) Public() map[string]any {
	return map[string]any{
		"mediaId":   r.MediaID,
		"fileName":  r.FileName,
		"mimeType":  r.MimeType,
		"fileSize":  r.FileSize,
		"sha256":    r.SHA256,
		"createdAt": r.CreatedAt,
	}
}

// MediaStorage gestiona el almacenamiento temporal de archivos con deduplicación por hash SHA-256.
type MediaStorage struct {
	mu       sync.RWMutex
	mediaDir string
	records  map[string]*MediaRecord
}

// NewMediaStorage crea un nuevo gestor de almacenamiento de medios en el directorio especificado.
func NewMediaStorage(mediaDir string) *MediaStorage {
	if mediaDir == "" {
		mediaDir = ".fbchat_media"
	}
	_ = os.MkdirAll(mediaDir, 0700)
	_ = permissions.SetPrivateFilePermissions(mediaDir)

	return &MediaStorage{
		mediaDir: mediaDir,
		records:  make(map[string]*MediaRecord),
	}
}

func sanitizeFilename(name string) string {
	cleaned := filenameSanitizer.ReplaceAllString(name, "_")
	if len(cleaned) > 200 {
		cleaned = cleaned[:200]
	}
	if cleaned == "" {
		cleaned = "unnamed_media"
	}
	return cleaned
}

// Store guarda un stream de datos en disco, deduplica por SHA-256 y registra los metadatos.
func (s *MediaStorage) Store(filename, mimeType string, r io.Reader) (*MediaRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	hasher := sha256.New()
	tempFile, err := os.CreateTemp(s.mediaDir, "upload_*.tmp")
	if err != nil {
		return nil, fmt.Errorf("error al crear archivo temporal: %w", err)
	}
	defer os.Remove(tempFile.Name())

	tee := io.TeeReader(r, hasher)
	size, err := io.Copy(tempFile, tee)
	_ = tempFile.Close()
	if err != nil {
		return nil, fmt.Errorf("error al escribir datos del archivo: %w", err)
	}

	hashStr := hex.EncodeToString(hasher.Sum(nil))
	mediaID := hashStr[:16]

	// Comprobar si ya existe por hash
	if existing, found := s.records[mediaID]; found {
		return existing, nil
	}

	cleanName := sanitizeFilename(filename)
	destPath := filepath.Join(s.mediaDir, mediaID+"_"+cleanName)

	if err := os.Rename(tempFile.Name(), destPath); err != nil {
		return nil, fmt.Errorf("error al ubicar archivo final: %w", err)
	}
	_ = permissions.SetPrivateFilePermissions(destPath)

	record := &MediaRecord{
		MediaID:   mediaID,
		FileName:  cleanName,
		MimeType:  mimeType,
		FileSize:  size,
		SHA256:    hashStr,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.records[mediaID] = record

	return record, nil
}

// GetInfo obtiene los metadatos del medio por su ID.
func (s *MediaStorage) GetInfo(mediaID string) (*MediaRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, exists := s.records[mediaID]
	if !exists {
		return nil, fmt.Errorf("medio no encontrado: %s", mediaID)
	}
	return rec, nil
}

// GetFilePath devuelve la ruta física al archivo del medio en disco.
func (s *MediaStorage) GetFilePath(mediaID string) (string, error) {
	rec, err := s.GetInfo(mediaID)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.mediaDir, rec.MediaID+"_"+rec.FileName), nil
}

// ListAll lista todos los medios registrados hasta un límite dado.
func (s *MediaStorage) ListAll(limit int) []map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []map[string]any
	count := 0
	for _, rec := range s.records {
		results = append(results, rec.Public())
		count++
		if limit > 0 && count >= limit {
			break
		}
	}
	if results == nil {
		results = []map[string]any{}
	}
	return results
}

// Delete elimina un medio del registro y del disco.
func (s *MediaStorage) Delete(mediaID string) bool {
	s.mu.Lock()
	rec, exists := s.records[mediaID]
	if !exists {
		s.mu.Unlock()
		return false
	}
	delete(s.records, mediaID)
	s.mu.Unlock()

	filePath := filepath.Join(s.mediaDir, rec.MediaID+"_"+rec.FileName)
	_ = os.Remove(filePath)
	return true
}
