package api

import (
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
	for i := range sessionsList {
		id, _ := sessionsList[i]["sessionId"].(string)
		if s.ListenerManager != nil {
			sessionsList[i]["listenerStatus"] = s.ListenerManager.GetStatus(id)
		} else {
			sessionsList[i]["listenerStatus"] = "idle"
		}
	}
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
	s.EventHub.EmitSessionStatus(rec.SessionID, rec.Status, "")
	pub := rec.Public()
	pub["listenerStatus"] = "idle"
	JSONSuccess(w, "Session created and validated successfully.", pub, http.StatusCreated)
}

func (s *Server) handleGetSessionStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}

	statusMap := rec.Detailed()
	if s.ListenerManager != nil {
		statusMap["isListening"] = s.ListenerManager.IsListening(sessionID)
		statusMap["listenerStatus"] = s.ListenerManager.GetStatus(sessionID)
	} else {
		statusMap["isListening"] = false
		statusMap["listenerStatus"] = "idle"
	}
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
	s.EventHub.EmitSessionStatus(sessionID, rec.Status, "")
	pub := rec.Public()
	if s.ListenerManager != nil {
		pub["listenerStatus"] = s.ListenerManager.GetStatus(sessionID)
	} else {
		pub["listenerStatus"] = "idle"
	}
	JSONSuccess(w, "Session re-validated successfully.", pub, http.StatusOK)
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
	s.EventHub.EmitSessionStatus(sessionID, "deleted", "")
	JSONSuccess(w, "Session deleted successfully.", map[string]string{"sessionId": sessionID}, http.StatusOK)
}

func (s *Server) handleStartListening(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}

	JSONError(w, "LISTENER_NOT_IMPLEMENTED", "The Messenger realtime transport is not available in this build.", http.StatusNotImplemented, map[string]any{"sessionId": rec.SessionID}, "")
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
