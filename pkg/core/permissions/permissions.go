package permissions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CreatePrivateJSONFile crea un archivo JSON con contenido inicial y permisos restrictivos si no existe.
func CreatePrivateJSONFile(path string, template any) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false, fmt.Errorf("no se pudo crear el directorio padre: %w", err)
	}

	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return false, fmt.Errorf("error al serializar plantilla json: %w", err)
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return false, fmt.Errorf("error al escribir archivo temporal: %w", err)
	}

	if err := SetPrivateFilePermissions(tempPath); err != nil {
		_ = os.Remove(tempPath)
		return false, fmt.Errorf("error al aplicar permisos privados al archivo temporal: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return false, fmt.Errorf("error al renombrar archivo seguro a destino: %w", err)
	}

	_ = SetPrivateFilePermissions(path)
	return true, nil
}
