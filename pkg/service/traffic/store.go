package traffic

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

// OnRecordCallback define la función invocada cada vez que se guarda un registro de tráfico.
type OnRecordCallback func(record *TrafficRecord)

// TrafficStore gestiona el almacenamiento en memoria con buffer circular de peticiones y respuestas.
type TrafficStore struct {
	mu           sync.RWMutex
	capacity     int
	records      []*TrafficRecord
	recordsMap   map[string]*TrafficRecord
	counter      atomic.Int64
	onRecordHook OnRecordCallback
}

// NewTrafficStore inicializa un nuevo almacén de tráfico con la capacidad máxima especificada.
func NewTrafficStore(capacity int) *TrafficStore {
	if capacity <= 0 {
		capacity = 250
	}
	return &TrafficStore{
		capacity:   capacity,
		records:    make([]*TrafficRecord, 0, capacity),
		recordsMap: make(map[string]*TrafficRecord),
	}
}

// SetOnRecordHook asigna un callback para notificaciones en tiempo real (p.ej. WebSocket).
func (s *TrafficStore) SetOnRecordHook(hook OnRecordCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onRecordHook = hook
}

// NextID genera un identificador único incremental para el registro de tráfico.
func (s *TrafficStore) NextID() string {
	val := s.counter.Add(1)
	return fmt.Sprintf("req_%05d", val)
}

// Record guarda una nueva petición en el almacén circular y despacha el hook en tiempo real.
func (s *TrafficStore) Record(rec *TrafficRecord) {
	if rec == nil {
		return
	}
	if rec.ID == "" {
		rec.ID = s.NextID()
	}
	if rec.Timestamp == "" {
		rec.Timestamp = UTCNow()
	}
	if rec.ReqHeaders == nil {
		rec.ReqHeaders = rec.RequestHeaders
	}
	if rec.RespHeaders == nil {
		rec.RespHeaders = rec.ResponseHeaders
	}
	if rec.ReqBody == "" {
		rec.ReqBody = rec.RequestBody
	}
	if rec.RespBody == "" {
		rec.RespBody = rec.ResponseBody
	}
	if rec.ReqBodySize == 0 && rec.RequestBody != "" {
		rec.ReqBodySize = len(rec.RequestBody)
	}
	if rec.RespBodySize == 0 && rec.ResponseBody != "" {
		rec.RespBodySize = len(rec.ResponseBody)
	}

	var hook OnRecordCallback
	s.mu.Lock()
	// Si alcanzamos la capacidad máxima, eliminar el registro más antiguo
	if len(s.records) >= s.capacity {
		oldest := s.records[0]
		delete(s.recordsMap, oldest.ID)
		s.records = s.records[1:]
	}
	s.records = append(s.records, rec)
	s.recordsMap[rec.ID] = rec
	hook = s.onRecordHook
	s.mu.Unlock()

	if hook != nil {
		hook(rec)
	}
}

// List devuelve las peticiones registradas aplicando filtros opcionales de dirección y búsqueda.
func (s *TrafficStore) List(direction string, search string, limit int) []*TrafficRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dirFilter := strings.ToUpper(strings.TrimSpace(direction))
	searchFilter := strings.ToLower(strings.TrimSpace(search))

	// Iterar en orden inverso (más recientes primero)
	var filtered []*TrafficRecord
	for i := len(s.records) - 1; i >= 0; i-- {
		r := s.records[i]

		if dirFilter != "" && dirFilter != "ALL" {
			if string(r.Direction) != dirFilter {
				continue
			}
		}

		if searchFilter != "" {
			matches := strings.Contains(strings.ToLower(r.URL), searchFilter) ||
				strings.Contains(strings.ToLower(r.Method), searchFilter) ||
				strings.Contains(strings.ToLower(r.RequestBody), searchFilter) ||
				strings.Contains(strings.ToLower(r.ResponseBody), searchFilter)
			if !matches {
				continue
			}
		}

		filtered = append(filtered, r)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}

	if filtered == nil {
		filtered = []*TrafficRecord{}
	}
	return filtered
}

// Get recupera los detalles completos de una petición por su ID.
func (s *TrafficStore) Get(id string) (*TrafficRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.recordsMap[id]
	return rec, ok
}

// Clear vacía todo el historial de tráfico.
func (s *TrafficStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = make([]*TrafficRecord, 0, s.capacity)
	s.recordsMap = make(map[string]*TrafficRecord)
}

// Stats calcula las métricas consolidadas del tráfico en memoria.
func (s *TrafficStore) Stats() TrafficStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stats TrafficStats
	stats.TotalRequests = len(s.records)
	if len(s.records) == 0 {
		return stats
	}

	var totalDuration int64
	for _, r := range s.records {
		if r.Direction == DirectionOutbound {
			stats.OutboundFB++
		} else {
			stats.InboundAPI++
		}
		if r.StatusCode >= 400 || r.Error != "" {
			stats.ErrorCount++
		}
		totalDuration += r.DurationMs
	}

	stats.AvgDurationMs = float64(totalDuration) / float64(len(s.records))
	stats.LastActivityUTC = s.records[len(s.records)-1].Timestamp
	return stats
}
