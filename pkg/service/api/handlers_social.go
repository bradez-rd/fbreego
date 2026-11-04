package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/features/social"
	"github.com/bradez-rd/fbreego/pkg/messaging/attachments"
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

func (s *Server) handleSearchMarketplace(w http.ResponseWriter, r *http.Request) {
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

	q := r.URL.Query().Get("q")
	if q == "" {
		q = r.URL.Query().Get("query")
	}

	lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lng, _ := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
	radius, _ := strconv.Atoi(r.URL.Query().Get("radius"))
	minPrice, _ := strconv.ParseInt(r.URL.Query().Get("minPrice"), 10, 64)
	maxPrice, _ := strconv.ParseInt(r.URL.Query().Get("maxPrice"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	cursor := r.URL.Query().Get("cursor")

	params := social.MarketplaceSearchParams{
		Query:     q,
		Latitude:  lat,
		Longitude: lng,
		RadiusKm:  radius,
		MinPrice:  minPrice,
		MaxPrice:  maxPrice,
		Limit:     limit,
		Cursor:    cursor,
	}

	results, err := social.SearchMarketplace(r.Context(), s.httpClient, rec.ProviderData, params)
	if err != nil {
		JSONError(w, "MARKETPLACE_SEARCH_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	JSONSuccess(w, "Marketplace search executed successfully.", results, http.StatusOK)
}

func (s *Server) handleCreateMarketplaceItem(w http.ResponseWriter, r *http.Request) {
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

	var input social.MarketplaceItemInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "Invalid request payload.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	if strings.TrimSpace(input.Title) == "" {
		JSONError(w, "VALIDATION_FAILED", "Title is required for marketplace listings.", http.StatusBadRequest, nil, "")
		return
	}

	// Cargar medios locales a Facebook si fueron suministrados
	for _, mID := range input.MediaIDs {
		if s.MediaStore != nil {
			if filePath, err := s.MediaStore.GetFilePath(mID); err == nil {
				if f, err := os.Open(filePath); err == nil {
					info, _ := s.MediaStore.GetInfo(mID)
					fileName := "attachment.jpg"
					if info != nil && info.FileName != "" {
						fileName = info.FileName
					}
					uploaded, upErr := attachments.Upload(r.Context(), s.httpClient, rec.ProviderData, fileName, f, false)
					_ = f.Close()
					if upErr == nil && uploaded != nil && uploaded.AttachmentID != "" {
						input.ImageIDs = append(input.ImageIDs, uploaded.AttachmentID)
					}
				}
			}
		}
	}

	res, err := social.CreateMarketplaceItem(r.Context(), s.httpClient, rec.ProviderData, input)
	if err != nil {
		JSONError(w, "MARKETPLACE_PUBLISH_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	s.RecordAudit("marketplace_publish", fmt.Sprintf("Marketplace listing created for session %s: %s", sessionID, input.Title), sessionID)
	JSONSuccess(w, "Marketplace listing created successfully.", res, http.StatusCreated)
}
