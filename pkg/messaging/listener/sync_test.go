package listener

import (
	"testing"
)

func TestSyncManager(t *testing.T) {
	sm := NewSyncManager(100)
	if sm.GetLastSeqID() != 100 {
		t.Errorf("expected 100, got %d", sm.GetLastSeqID())
	}

	// Update with smaller or equal seq_id should be ignored
	if sm.UpdateSeqID(99) {
		t.Errorf("expected UpdateSeqID(99) to return false")
	}
	if sm.UpdateSeqID(100) {
		t.Errorf("expected UpdateSeqID(100) to return false")
	}
	if sm.GetLastSeqID() != 100 {
		t.Errorf("expected seq_id to remain 100")
	}

	// Update with larger seq_id should succeed
	if !sm.UpdateSeqID(105) {
		t.Errorf("expected UpdateSeqID(105) to return true")
	}
	if sm.GetLastSeqID() != 105 {
		t.Errorf("expected seq_id to be 105, got %d", sm.GetLastSeqID())
	}
}
