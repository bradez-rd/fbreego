package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/bradez-rd/fbreego/pkg/core/permissions"
)

// FileSessionStorage implementa SessionStorage leyendo y escribiendo en un archivo JSON.
type FileSessionStorage struct {
	mu       sync.Mutex
	filePath string
	key      string
}

// NewFileSessionStorage crea una instancia de FileSessionStorage.
func NewFileSessionStorage(filePath string, key string) *FileSessionStorage {
	if filePath == "" {
		filePath = "config.json"
	}
	if key == "" {
		key = "cookies"
	}
	return &FileSessionStorage{
		filePath: filePath,
		key:      key,
	}
}

func (f *FileSessionStorage) readData() (map[string]any, error) {
	data := make(map[string]any)
	raw, err := os.ReadFile(f.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return data, nil
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return make(map[string]any), nil
	}
	return data, nil
}

func (f *FileSessionStorage) writeData(data map[string]any) error {
	dir := filepath.Dir(f.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tempPath := f.filePath + ".tmp"
	if err := os.WriteFile(tempPath, raw, 0600); err != nil {
		return err
	}
	_ = permissions.SetPrivateFilePermissions(tempPath)
	if err := os.Rename(tempPath, f.filePath); err != nil {
		return err
	}
	_ = permissions.SetPrivateFilePermissions(f.filePath)
	return nil
}

// Load carga la cadena de cookies desde el archivo JSON.
func (f *FileSessionStorage) Load() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := f.readData()
	if err != nil {
		return "", err
	}
	val, ok := data[f.key]
	if !ok || val == nil {
		return "", nil
	}
	cookieStr, ok := val.(string)
	if !ok {
		return fmt.Sprintf("%v", val), nil
	}
	return cookieStr, nil
}

// Save guarda la cadena de cookies en el archivo JSON.
func (f *FileSessionStorage) Save(cookies string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := f.readData()
	if err != nil {
		data = make(map[string]any)
	}
	data[f.key] = cookies
	return f.writeData(data)
}

// Clear borra la clave de cookies del archivo JSON.
func (f *FileSessionStorage) Clear() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := f.readData()
	if err != nil {
		return nil
	}
	delete(data, f.key)
	return f.writeData(data)
}
