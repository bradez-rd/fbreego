package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMemorySessionStorage(t *testing.T) {
	mem := NewMemorySessionStorage("")
	val, err := mem.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}

	testCookie := "c_user=123; xs=abc;"
	if err := mem.Save(testCookie); err != nil {
		t.Fatalf("save error: %v", err)
	}

	val, err = mem.Load()
	if err != nil || val != testCookie {
		t.Errorf("expected %q, got %q", testCookie, val)
	}

	if err := mem.Clear(); err != nil {
		t.Fatalf("clear error: %v", err)
	}
	val, _ = mem.Load()
	if val != "" {
		t.Errorf("expected empty after clear, got %q", val)
	}
}

func TestEnvSessionStorage(t *testing.T) {
	envKey := "TEST_FB_SESSION_COOKIES"
	os.Setenv(envKey, "c_user=999; xs=env_secret;")
	defer os.Unsetenv(envKey)

	storage := NewEnvSessionStorage(envKey)
	val, err := storage.Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if val != "c_user=999; xs=env_secret;" {
		t.Errorf("unexpected loaded env: %q", val)
	}

	if err := storage.Save("c_user=888;"); err != nil {
		t.Fatalf("save error: %v", err)
	}
	val, _ = storage.Load()
	if val != "c_user=888;" {
		t.Errorf("unexpected updated env: %q", val)
	}

	if err := storage.Clear(); err != nil {
		t.Fatalf("clear error: %v", err)
	}
	val, _ = storage.Load()
	if val != "" {
		t.Errorf("expected empty env, got %q", val)
	}
}

func TestFileSessionStorage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fbreego_storage_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "session_test.json")
	storage := NewFileSessionStorage(filePath, "my_cookies")

	// Initially empty
	val, err := storage.Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty, got %q", val)
	}

	// Save
	testCookie := "c_user=456; xs=def;"
	if err := storage.Save(testCookie); err != nil {
		t.Fatalf("save error: %v", err)
	}

	val, err = storage.Load()
	if err != nil || val != testCookie {
		t.Errorf("expected %q, got %q", testCookie, val)
	}

	// Clear
	if err := storage.Clear(); err != nil {
		t.Fatalf("clear error: %v", err)
	}
	val, _ = storage.Load()
	if val != "" {
		t.Errorf("expected empty after clear, got %q", val)
	}
}
