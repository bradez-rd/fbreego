//go:build windows

package permissions

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
)

// SetPrivateFilePermissions grants full control only to the current Windows principal.
func SetPrivateFilePermissions(path string) error {
	current, err := user.Current()
	if err != nil || current.Uid == "" {
		return fmt.Errorf("no se pudo resolver la identidad de Windows: %w", err)
	}

	// A SID avoids ambiguous local/domain account names. OI/CI ensures files and
	// subdirectories created later remain accessible to the same principal.
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("no se pudo inspeccionar la ruta para aplicar ACL: %w", err)
	}
	grantArg := fmt.Sprintf("*%s:F", current.Uid)
	if info.IsDir() {
		grantArg = fmt.Sprintf("*%s:(OI)(CI)F", current.Uid)
	}
	cmd := exec.Command("icacls.exe", path, "/inheritance:r", "/grant:r", grantArg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("icacls falló: %w: %s", err, string(out))
	}
	return nil
}
