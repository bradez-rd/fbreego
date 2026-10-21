package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListTraffic(w http.ResponseWriter, r *http.Request) {
	direction := r.URL.Query().Get("direction")
	search := r.URL.Query().Get("search")
	limitStr := r.URL.Query().Get("limit")

	limit := 100
	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 {
			limit = val
		}
	}

	records := s.TrafficStore.List(direction, search, limit)
	JSONSuccess(w, "Traffic history retrieved.", records, http.StatusOK)
}

func (s *Server) handleGetTrafficStats(w http.ResponseWriter, r *http.Request) {
	stats := s.TrafficStore.Stats()
	JSONSuccess(w, "Traffic stats retrieved.", stats, http.StatusOK)
}

func (s *Server) handleGetTrafficDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, found := s.TrafficStore.Get(id)
	if !found {
		JSONError(w, "TRAFFIC_NOT_FOUND", "Petición no encontrada en el registro de tráfico.", http.StatusNotFound, nil, "")
		return
	}
	JSONSuccess(w, "Traffic record details retrieved.", rec, http.StatusOK)
}

func (s *Server) handleClearTraffic(w http.ResponseWriter, r *http.Request) {
	s.TrafficStore.Clear()
	s.RecordAudit("traffic_clear", "Historial de tráfico HTTP vaciado manualmente", "")
	JSONSuccess(w, "Historial de tráfico limpiado correctamente.", map[string]any{"cleared": true}, http.StatusOK)
}
