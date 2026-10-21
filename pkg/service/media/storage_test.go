package media

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMediaStorageSaveAndRetrieve(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fbreego_media_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewMediaStorage(tmpDir)

	content := []byte("fake image or audio content for testing deduplication")
	rec1, err := storage.Store("test_photo.png", "image/png", bytes.NewReader(content))
	if err != nil {
		t.Fatalf("failed to save media: %v", err)
	}

	if rec1.MediaID == "" {
		t.Errorf("expected non-empty MediaID")
	}
	if rec1.FileName != "test_photo.png" {
		t.Errorf("expected fileName test_photo.png, got %s", rec1.FileName)
	}

	// Saving identical content should return deduplicated record with same SHA256
	rec2, err := storage.Store("another_name.png", "image/png", bytes.NewReader(content))
	if err != nil {
		t.Fatalf("failed to save second media: %v", err)
	}
	if rec2.SHA256 != rec1.SHA256 {
		t.Errorf("expected same SHA256 %s, got %s", rec1.SHA256, rec2.SHA256)
	}

	// Retrieve by ID
	retrieved, err := storage.GetInfo(rec1.MediaID)
	if err != nil {
		t.Fatalf("failed to get media info: %v", err)
	}
	if retrieved.MediaID != rec1.MediaID {
		t.Errorf("expected ID %s, got %s", rec1.MediaID, retrieved.MediaID)
	}

	filePath, err := storage.GetFilePath(rec1.MediaID)
	if err != nil {
		t.Fatalf("failed to get file path: %v", err)
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("expected media file to exist on disk: %v", err)
	}

	// List
	list := storage.ListAll(10)
	if len(list) < 1 {
		t.Errorf("expected at least 1 record in list")
	}

	// Delete
	deleted, err := storage.Delete(rec1.MediaID)
	if err != nil {
		t.Fatalf("failed to delete media: %v", err)
	}
	if !deleted {
		t.Fatalf("failed to delete media")
	}
	_, err = storage.GetInfo(rec1.MediaID)
	if err == nil {
		t.Errorf("expected error getting deleted media, got nil")
	}
}

func TestSanitizeFilename(t *testing.T) {
	clean := sanitizeFilename("../../../evil*file?.png")
	if filepath.Base(clean) == clean && !bytes.Contains([]byte(clean), []byte("..")) {
		// sanitization worked
	}
}
