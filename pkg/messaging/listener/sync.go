package listener

import (
	"sync/atomic"
)

// DeltaMessage representa un mensaje entrante decodificado desde el broker MQTT de Facebook.
type DeltaMessage struct {
	MessageID   string `json:"messageId"`
	ThreadID    string `json:"threadId"`
	SenderID    string `json:"senderId"`
	Body        string `json:"body"`
	TimestampMs int64  `json:"timestampMs"`
	IsGroup     bool   `json:"isGroup"`
}

// SyncManager gestiona atómicamente el número de secuencia seq_id para evitar mensajes duplicados o saltos en la sincronización.
type SyncManager struct {
	lastSeqID atomic.Int64
}

// NewSyncManager crea un nuevo gestor de secuencias inicializado.
func NewSyncManager(initialSeqID int64) *SyncManager {
	s := &SyncManager{}
	s.lastSeqID.Store(initialSeqID)
	return s
}

// GetLastSeqID devuelve el último número de secuencia registrado.
func (s *SyncManager) GetLastSeqID() int64 {
	return s.lastSeqID.Load()
}

// UpdateSeqID actualiza el seq_id si el nuevo valor es mayor al actual.
func (s *SyncManager) UpdateSeqID(newSeqID int64) bool {
	for {
		current := s.lastSeqID.Load()
		if newSeqID <= current {
			return false
		}
		if s.lastSeqID.CompareAndSwap(current, newSeqID) {
			return true
		}
	}
}
