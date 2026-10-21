package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/bradez-rd/fbreego/pkg/core/lightspeed"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/features/thread"
)

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenantId")
	details := r.URL.Query().Get("details") == "true" || r.URL.Query().Get("details") == "1"

	sessionsList := s.Registry.ListPublic(tenantID, details)
	JSONSuccess(w, "Sessions retrieved successfully.", sessionsList, http.StatusOK)
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var input types.SessionCreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	rec, err := s.Registry.Create(r.Context(), input)
	if err != nil {
		JSONError(w, "SESSION_VALIDATION_FAILED", err.Error(), http.StatusBadRequest, nil, "")
		return
	}

	s.RecordAudit("session_create", fmt.Sprintf("Session %s created", rec.SessionID), rec.SessionID)
	JSONSuccess(w, "Session created and validated successfully.", rec.Public(), http.StatusCreated)
}

func (s *Server) handleGetSessionStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}

	statusMap := rec.Public()
	statusMap["isListening"] = s.ListenerManager.IsListening(sessionID)
	JSONSuccess(w, "Session status retrieved.", statusMap, http.StatusOK)
}

func (s *Server) handleValidateSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	rec, err := s.Registry.Validate(r.Context(), sessionID)
	if err != nil {
		s.RecordAudit("session_validate_failed", err.Error(), sessionID)
		JSONError(w, "SESSION_INVALID", err.Error(), http.StatusBadRequest, nil, "")
		return
	}

	s.RecordAudit("session_validate_success", "Session re-validated with Facebook", sessionID)
	JSONSuccess(w, "Session re-validated successfully.", rec.Public(), http.StatusOK)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	s.ListenerManager.StopListening(sessionID)
	err := s.Registry.Delete(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}

	s.RecordAudit("session_delete", "Session deleted", sessionID)
	JSONSuccess(w, "Session deleted successfully.", map[string]string{"sessionId": sessionID}, http.StatusOK)
}

func (s *Server) handleStartListening(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}

	err = s.ListenerManager.StartListening(r.Context(), sessionID, func(ctx context.Context) {
		s.Registry.RecordActivity(sessionID)
	})
	if err != nil {
		JSONError(w, "LISTENER_ALREADY_ACTIVE", err.Error(), http.StatusConflict, nil, "")
		return
	}

	s.RecordAudit("listener_start", "Listener started", sessionID)
	JSONSuccess(w, "Listener started successfully.", map[string]any{"sessionId": rec.SessionID, "status": "listening"}, http.StatusOK)
}

func (s *Server) handleStopListening(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	stopped := s.ListenerManager.StopListening(sessionID)
	if !stopped {
		JSONError(w, "LISTENER_NOT_FOUND", "No active listener for this session.", http.StatusNotFound, nil, "")
		return
	}

	s.RecordAudit("listener_stop", "Listener stopped", sessionID)
	JSONSuccess(w, "Listener stopped successfully.", map[string]any{"sessionId": sessionID, "status": "stopped"}, http.StatusOK)
}

func (s *Server) handleGetListenStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	isListening := s.ListenerManager.IsListening(sessionID)
	status := s.ListenerManager.GetStatus(sessionID)
	JSONSuccess(w, "Listener status retrieved.", map[string]any{
		"sessionId":   sessionID,
		"isListening": isListening,
		"status":      status,
	}, http.StatusOK)
}

func (s *Server) handleListThreads(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}

	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have provider data.", http.StatusBadRequest, nil, "")
		return
	}

	threadsData, err := thread.FetchAllThreads(r.Context(), s.httpClient, rec.ProviderData)
	if err != nil {
		// Fallback con LightSpeed HTML parser
		lsData, _ := lightspeed.FetchLightSpeedData(r.Context(), s.httpClient, rec.BuildCookieHeader())
		JSONSuccess(w, "Threads retrieved via LightSpeed fallback.", lsData, http.StatusOK)
		return
	}

	JSONSuccess(w, "Threads retrieved successfully.", threadsData, http.StatusOK)
}
