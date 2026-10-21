package storage

import (
	"os"
	"sync"
)

// EnvSessionStorage implementa SessionStorage leyendo y escribiendo en una variable de entorno.
type EnvSessionStorage struct {
	envVar string
}

// NewEnvSessionStorage crea una instancia de EnvSessionStorage.
func NewEnvSessionStorage(envVar string) *EnvSessionStorage {
	if envVar == "" {
		envVar = "FB_COOKIES"
	}
	return &EnvSessionStorage{envVar: envVar}
}

// Load obtiene las cookies de la variable de entorno.
func (e *EnvSessionStorage) Load() (string, error) {
	return os.Getenv(e.envVar), nil
}

// Save establece el valor de la variable de entorno en el proceso actual.
func (e *EnvSessionStorage) Save(cookies string) error {
	return os.Setenv(e.envVar, cookies)
}

// Clear vacía el valor de la variable de entorno.
func (e *EnvSessionStorage) Clear() error {
	return os.Unsetenv(e.envVar)
}

// MemorySessionStorage implementa SessionStorage en memoria para pruebas o usos efímeros.
type MemorySessionStorage struct {
	mu      sync.RWMutex
	cookies string
}

// NewMemorySessionStorage crea una instancia de almacenamiento en memoria.
func NewMemorySessionStorage(initialCookies string) *MemorySessionStorage {
	return &MemorySessionStorage{cookies: initialCookies}
}

func (m *MemorySessionStorage) Load() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cookies, nil
}

func (m *MemorySessionStorage) Save(cookies string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cookies = cookies
	return nil
}

func (m *MemorySessionStorage) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cookies = ""
	return nil
}
