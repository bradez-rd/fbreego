package config

import (
	"path/filepath"
	"testing"
)

func TestValueReadsFbreegoVariable(t *testing.T) {
	t.Setenv("FBREEGO_TEST_VALUE", "new")
	if got := Value("FBREEGO_TEST_VALUE"); got != "new" {
		t.Fatalf("expected canonical value, got %q", got)
	}
}

func TestDataDirUsesConfiguredOrDefaultPath(t *testing.T) {
	root := t.TempDir()
	preferred := filepath.Join(root, ".fbreego_sessions")
	if got := DataDir("FBREEGO_TEST_DIR", preferred); got != preferred {
		t.Fatalf("expected preferred directory, got %q", got)
	}
	configured := filepath.Join(root, "configured")
	t.Setenv("FBREEGO_TEST_DIR", configured)
	if got := DataDir("FBREEGO_TEST_DIR", preferred); got != configured {
		t.Fatalf("expected configured directory, got %q", got)
	}
}
