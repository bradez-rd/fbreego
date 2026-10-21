//go:build !windows

package permissions

import (
	"os"
)

// SetPrivateFilePermissions establece permisos 0600 en sistemas operativos basados en POSIX.
func SetPrivateFilePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.Chmod(path, 0700)
	}
	return os.Chmod(path, 0600)
}
