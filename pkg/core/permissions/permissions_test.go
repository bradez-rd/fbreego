package permissions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreatePrivateJSONFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fbreego_perm_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetFile := filepath.Join(tmpDir, "sub", "config.json")
	template := map[string]string{"sample": "data"}

	created, err := CreatePrivateJSONFile(targetFile, template)
	if err != nil {
		t.Fatalf("expected successful creation, got: %v", err)
	}
	if !created {
		t.Errorf("expected created to be true for new file")
	}

	// Calling again should return false (already exists) without error
	created2, err := CreatePrivateJSONFile(targetFile, template)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if created2 {
		t.Errorf("expected created to be false for existing file")
	}

	// Verify SetPrivateFilePermissions works on the file
	if err := SetPrivateFilePermissions(targetFile); err != nil {
		t.Errorf("SetPrivateFilePermissions failed: %v", err)
	}
}
