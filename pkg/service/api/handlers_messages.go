package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/features/thread"
	"github.com/bradez-rd/fbreego/pkg/messaging/attachments"
	"github.com/bradez-rd/fbreego/pkg/messaging/edit"
	"github.com/bradez-rd/fbreego/pkg/messaging/reactions"
	"github.com/bradez-rd/fbreego/pkg/messaging/receipts"
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

	var res map[string]any
	if input.ThreadType == "e2ee" || strings.HasSuffix(input.ThreadID, "@msgr") {
		if s.E2EEManager != nil && s.E2EEManager.Available() {
			chatJID := input.ThreadID
			if !strings.HasSuffix(chatJID, "@msgr") {
				chatJID = chatJID + "@msgr"
			}
			res, err = s.E2EEManager.Call(r.Context(), "sendE2EETextMessage", map[string]any{
				"chatJid": chatJID,
				"text":    input.Text,
				"session": rec.ProviderData,
			})
		} else {
			JSONError(w, "E2EE_UNAVAILABLE", "El backend E2EE no está disponible en este momento.", http.StatusServiceUnavailable, nil, "")
			return
		}
	} else {
		res, err = send.SendText(r.Context(), s.httpClient, rec.ProviderData, input)
	}

	if err != nil {
		if errors.Is(err, send.ErrCutoverToE2EE) {
			JSONError(w, "E2EE_CUTOVER_REQUIRED", "Meta exige cifrado E2EE para esta conversación. Envíe el mensaje con threadType='e2ee'.", http.StatusConflict, map[string]any{
				"threadId": input.ThreadID,
				"required": "e2ee",
			}, "")
			return
		}
		JSONError(w, "SEND_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	s.Registry.RecordActivity(sessionID)
	s.EventHub.Publish(map[string]any{"event": "message_sent", "sessionId": sessionID, "result": res})
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
	s.EventHub.Publish(map[string]any{"event": "reaction_updated", "sessionId": sessionID, "messageId": input.MessageID, "action": input.Action})
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
	s.EventHub.Publish(map[string]any{"event": "message_unsent", "sessionId": sessionID, "messageId": input.MessageID})
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
	if len(input.MediaIDs) == 0 {
		JSONError(w, "INVALID_REQUEST", "At least one mediaId is required.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	// Cargar y adjuntar medios registrados
	var uploadedResults []*types.AttachmentResult
	for _, mID := range input.MediaIDs {
		filePath, err := s.MediaStore.GetFilePath(mID)
		if err != nil {
			JSONError(w, "MEDIA_NOT_FOUND", err.Error(), http.StatusBadRequest, map[string]any{"mediaId": mID}, "")
			return
		}
		f, err := os.Open(filePath)
		if err != nil {
			JSONError(w, "MEDIA_OPEN_FAILED", err.Error(), http.StatusInternalServerError, map[string]any{"mediaId": mID}, "")
			return
		}
		info, err := s.MediaStore.GetInfo(mID)
		if err != nil {
			_ = f.Close()
			JSONError(w, "MEDIA_NOT_FOUND", err.Error(), http.StatusBadRequest, map[string]any{"mediaId": mID}, "")
			return
		}
		isVoiceClip := false
		if strings.HasPrefix(strings.ToLower(info.MimeType), "audio/") || strings.HasSuffix(strings.ToLower(info.FileName), ".mp3") || strings.HasSuffix(strings.ToLower(info.FileName), ".ogg") || strings.HasSuffix(strings.ToLower(info.FileName), ".m4a") {
			isVoiceClip = true
		}
		uploaded, err := attachments.Upload(r.Context(), s.httpClient, rec.ProviderData, info.FileName, f, isVoiceClip)
		_ = f.Close()
		if err != nil || uploaded == nil {
			if err == nil {
				err = fmt.Errorf("facebook no devolvió metadata del adjunto")
			}
			JSONError(w, "MEDIA_UPLOAD_FAILED", err.Error(), http.StatusBadGateway, map[string]any{"mediaId": mID}, "")
			return
		}
		uploadedResults = append(uploadedResults, uploaded)
	}

	// Envío de texto o confirmación de adjuntos
	msgInput := types.TextMessageInput{
		ThreadID:   input.ThreadID,
		Text:       input.Text,
		ThreadType: input.ThreadType,
	}
	res, err := send.SendWithAttachments(r.Context(), s.httpClient, rec.ProviderData, msgInput, uploadedResults)
	if err != nil {
		JSONError(w, "SEND_MEDIA_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	s.Registry.RecordActivity(sessionID)
	s.EventHub.Publish(map[string]any{"event": "message_sent", "sessionId": sessionID, "result": res, "attachments": uploadedResults})
	JSONSuccess(w, "Media message sent successfully.", map[string]any{
		"result":      res,
		"attachments": uploadedResults,
	}, http.StatusOK)
}

func (s *Server) handleGetThreadMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	threadID := chi.URLParam(r, "threadId")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}
	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have active provider data.", http.StatusBadRequest, nil, "")
		return
	}

	limit := 20
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	before := r.URL.Query().Get("before")

	msgs, err := thread.FetchThreadMessages(r.Context(), s.httpClient, rec.ProviderData, threadID, limit, before)
	if err != nil {
		JSONError(w, "FETCH_MESSAGES_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	s.Registry.RecordActivity(sessionID)
	JSONSuccess(w, "Thread messages retrieved.", map[string]any{
		"items":    msgs,
		"count":    len(msgs),
		"threadId": threadID,
	}, http.StatusOK)
}

func (s *Server) handleMarkThreadRead(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	threadID := chi.URLParam(r, "threadId")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}
	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have active provider data.", http.StatusBadRequest, nil, "")
		return
	}

	var body map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	var watermark int64
	if body != nil {
		if wm, ok := body["watermark"].(float64); ok {
			watermark = int64(wm)
		}
	}

	res, err := receipts.MarkRead(r.Context(), s.httpClient, rec.ProviderData, threadID, watermark)
	if err != nil {
		JSONError(w, "MARK_READ_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	s.Registry.RecordActivity(sessionID)
	s.EventHub.Publish(map[string]any{"event": "message.read", "sessionId": sessionID, "threadId": threadID, "watermark": watermark})
	JSONSuccess(w, "Thread marked as read.", res, http.StatusOK)
}

func (s *Server) handleMarkThreadDelivered(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	threadID := chi.URLParam(r, "threadId")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}
	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have active provider data.", http.StatusBadRequest, nil, "")
		return
	}

	var body map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	var watermark int64
	if body != nil {
		if wm, ok := body["watermark"].(float64); ok {
			watermark = int64(wm)
		}
	}

	res, err := receipts.MarkDelivered(r.Context(), s.httpClient, rec.ProviderData, threadID, watermark)
	if err != nil {
		JSONError(w, "MARK_DELIVERED_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	s.Registry.RecordActivity(sessionID)
	s.EventHub.Publish(map[string]any{"event": "message.delivered", "sessionId": sessionID, "threadId": threadID, "watermark": watermark})
	JSONSuccess(w, "Thread marked as delivered.", res, http.StatusOK)
}

func (s *Server) handleSendTyping(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	threadID := chi.URLParam(r, "threadId")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}
	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have active provider data.", http.StatusBadRequest, nil, "")
		return
	}

	var body map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	isTyping := true
	if body != nil {
		if typ, ok := body["typing"].(bool); ok {
			isTyping = typ
		} else if isTyp, ok := body["isTyping"].(bool); ok {
			isTyping = isTyp
		} else if st, ok := body["status"].(string); ok && st == "stopped" {
			isTyping = false
		}
	}

	res, err := receipts.SendTyping(r.Context(), s.httpClient, rec.ProviderData, threadID, isTyping)
	if err != nil {
		JSONError(w, "TYPING_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	s.Registry.RecordActivity(sessionID)
	eventName := "typing.started"
	if !isTyping {
		eventName = "typing.stopped"
	}
	s.EventHub.Publish(map[string]any{"event": eventName, "sessionId": sessionID, "threadId": threadID})
	JSONSuccess(w, "Typing indicator sent.", res, http.StatusOK)
}

func (s *Server) handleEditMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	messageID := chi.URLParam(r, "messageId")
	rec, err := s.Registry.Get(sessionID)
	if err != nil {
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}
	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have active provider data.", http.StatusBadRequest, nil, "")
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, "INVALID_REQUEST", "Invalid request payload.", http.StatusUnprocessableEntity, nil, "")
		return
	}
	newText, _ := body["text"].(string)
	if newText == "" {
		JSONError(w, "INVALID_REQUEST", "Field 'text' is required.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	res, err := edit.EditMessage(r.Context(), s.httpClient, rec.ProviderData, messageID, newText)
	if err != nil {
		JSONError(w, "EDIT_FAILED", err.Error(), http.StatusInternalServerError, nil, "")
		return
	}

	s.Registry.RecordActivity(sessionID)
	s.EventHub.Publish(map[string]any{"event": "message.edited", "sessionId": sessionID, "messageId": messageID, "text": newText})
	JSONSuccess(w, "Message edited successfully.", res, http.StatusOK)
}
