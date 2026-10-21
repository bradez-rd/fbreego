package api

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/messaging/attachments"
	"github.com/bradez-rd/fbreego/pkg/messaging/reactions"
	"github.com/bradez-rd/fbreego/pkg/messaging/send"
	"github.com/bradez-rd/fbreego/pkg/messaging/unsend"
)

func (s *Server) handleSendTextMessage(w http.ResponseWriter, r *http.Request) {
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

	var input types.TextMessageInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	res, err := send.SendText(r.Context(), s.httpClient, rec.ProviderData, input)
	if err != nil {
		JSONError(w, "SEND_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	s.Registry.RecordActivity(sessionID)
	JSONSuccess(w, "Message sent successfully.", res, http.StatusOK)
}

func (s *Server) handleReactMessage(w http.ResponseWriter, r *http.Request) {
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

	var input types.ReactionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	res, err := reactions.ReactMessage(r.Context(), s.httpClient, rec.ProviderData, input.MessageID, input.Emoji, input.Action)
	if err != nil {
		JSONError(w, "REACTION_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	s.Registry.RecordActivity(sessionID)
	JSONSuccess(w, "Reaction applied successfully.", res, http.StatusOK)
}

func (s *Server) handleUnsendMessage(w http.ResponseWriter, r *http.Request) {
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

	var input types.UnsendInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	res, err := unsend.UnsendMessage(r.Context(), s.httpClient, rec.ProviderData, input.MessageID)
	if err != nil {
		JSONError(w, "UNSEND_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	s.Registry.RecordActivity(sessionID)
	JSONSuccess(w, "Message unsend executed successfully.", res, http.StatusOK)
}

func (s *Server) handleSendMediaMessage(w http.ResponseWriter, r *http.Request) {
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

	var input types.MediaMessageInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	// Cargar y adjuntar medios registrados
	var uploadedIDs []string
	for _, mID := range input.MediaIDs {
		filePath, err := s.MediaStore.GetFilePath(mID)
		if err != nil {
			continue
		}
		f, err := os.Open(filePath)
		if err != nil {
			continue
		}
		info, _ := s.MediaStore.GetInfo(mID)
		uploaded, err := attachments.Upload(r.Context(), s.httpClient, rec.ProviderData, info.FileName, f, false)
		_ = f.Close()
		if err == nil && uploaded != nil {
			uploadedIDs = append(uploadedIDs, uploaded.AttachmentID)
		}
	}

	// Envío de texto o confirmación de adjuntos
	msgInput := types.TextMessageInput{
		ThreadID:   input.ThreadID,
		Text:       input.Text,
		ThreadType: input.ThreadType,
	}
	res, err := send.SendText(r.Context(), s.httpClient, rec.ProviderData, msgInput)
	if err != nil {
		JSONError(w, "SEND_MEDIA_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	s.Registry.RecordActivity(sessionID)
	JSONSuccess(w, "Media message sent successfully.", map[string]any{
		"result":        res,
		"attachmentIds": uploadedIDs,
	}, http.StatusOK)
}
