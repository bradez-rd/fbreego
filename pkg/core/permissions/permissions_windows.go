//go:build windows

package permissions

import (
	"fmt"
	"os"
	"os/exec"
)

// SetPrivateFilePermissions establece permisos de acceso exclusivo para el usuario actual en Windows usando icacls.
func SetPrivateFilePermissions(path string) error {
	username := os.Getenv("USERNAME")
	if username == "" {
		// Fallback si no está definida la variable de entorno
		return nil
	}

	// icacls path /inheritance:r /grant:r "%USERNAME%:(F)"
	grantArg := fmt.Sprintf("%s:(F)", username)
	cmd := exec.Command("icacls.exe", path, "/inheritance:r", "/grant:r", grantArg)
	if out, err := cmd.CombinedOutput(); err != nil {
		// No bloquear si icacls falla en ciertos entornos virtuales o CI restringidos
		_ = out
		return nil
	}

	return nil
}
