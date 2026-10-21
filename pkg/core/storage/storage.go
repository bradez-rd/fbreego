package storage

// SessionStorage es la interfaz base para persistir y cargar cadenas de cookies de sesión.
type SessionStorage interface {
	Load() (string, error)
	Save(cookies string) error
	Clear() error
}
