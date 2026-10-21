package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/features/social"
)

func (s *Server) handleGetUserInfo(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	userID := chi.URLParam(r, "userId")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}
	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have active provider data.", http.StatusBadRequest, nil, "")
		return
	}

	info, err := social.GetUserInfo(r.Context(), s.httpClient, rec.ProviderData, userID)
	if err != nil {
		JSONError(w, "USER_INFO_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	JSONSuccess(w, "User information retrieved.", info, http.StatusOK)
}

func (s *Server) handleSearchUsers(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	query := r.URL.Query().Get("q")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}
	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have active provider data.", http.StatusBadRequest, nil, "")
		return
	}

	results, err := social.SearchUsers(r.Context(), s.httpClient, rec.ProviderData, query)
	if err != nil {
		JSONError(w, "SEARCH_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	JSONSuccess(w, "Search results retrieved.", results, http.StatusOK)
}

func (s *Server) handleBlockUser(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}
	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have active provider data.", http.StatusBadRequest, nil, "")
		return
	}

	var input types.BlockInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	block := input.Action != "unblock"
	res, err := social.BlockUser(r.Context(), s.httpClient, rec.ProviderData, input.UserID, block)
	if err != nil {
		JSONError(w, "BLOCK_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	actionMsg := "User blocked successfully."
	if !block {
		actionMsg = "User unblocked successfully."
	}
	JSONSuccess(w, actionMsg, res, http.StatusOK)
}

func (s *Server) handleGetMarketplaceItem(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	itemID := chi.URLParam(r, "itemId")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}
	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have active provider data.", http.StatusBadRequest, nil, "")
		return
	}

	details, err := social.GetMarketplaceItem(r.Context(), s.httpClient, rec.ProviderData, itemID)
	if err != nil {
		JSONError(w, "MARKETPLACE_QUERY_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	JSONSuccess(w, "Marketplace item retrieved.", details, http.StatusOK)
}
