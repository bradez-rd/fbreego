package api

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/bradez-rd/fbreego/pkg/core/httputil"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/messaging/attachments"
	"github.com/bradez-rd/fbreego/pkg/messaging/reactions"
	"github.com/bradez-rd/fbreego/pkg/messaging/send"
	"github.com/bradez-rd/fbreego/pkg/messaging/unsend"
	"github.com/bradez-rd/fbreego/pkg/service/sessions"
)

func (s *Server) handleOperationError(w http.ResponseWriter, sessionID string, rec *sessions.SessionRecord, err error, defaultCode, defaultMsg string) {
	if rec != nil {
		rec.HandleHTTPResult(err)
	}
	s.RecordAudit(strings.ToLower(defaultCode)+"_failed", err.Error(), sessionID)
	if errors.Is(err, httputil.ErrAccountRestricted) {
		s.EventHub.EmitSessionStatus(sessionID, sessions.StatusBlocked, rec.LastError)
		JSONError(w, "ACCOUNT_RESTRICTED", "Facebook account is restricted, checkpointed, or blocked.", http.StatusForbidden, nil, "")
		return
	}
	if errors.Is(err, httputil.ErrSessionExpired) {
		s.EventHub.EmitSessionStatus(sessionID, sessions.StatusError, rec.LastError)
		JSONError(w, "SESSION_EXPIRED", "Session credentials expired or invalidated.", http.StatusUnauthorized, nil, "")
		return
	}
	if errors.Is(err, httputil.ErrRateLimited) {
		JSONError(w, "RATE_LIMITED", "Request was rate-limited by Facebook.", http.StatusTooManyRequests, nil, "")
		return
	}
	JSONError(w, defaultCode, defaultMsg, http.StatusInternalServerError, nil, "")
}

func (s *Server) handleSendTextMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	tenantID := extractTenantID(r)
	rec, err := s.Registry.GetForTenant(sessionID, tenantID)
	if err != nil {
		if strings.Contains(err.Error(), "acceso denegado") {
			JSONError(w, "FORBIDDEN", "Cross-tenant access is not allowed.", http.StatusForbidden, nil, "")
			return
		}
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}
	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have active provider data.", http.StatusBadRequest, nil, "")
		return
	}

	input, err := DecodeJSONBody[types.TextMessageInput](w, r, 64*1024)
	if err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid or exceeds allowed limits.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	if err := rec.AcquireExecutionSlot(r.Context()); err != nil {
		JSONError(w, "SESSION_EXECUTION_BLOCKED", err.Error(), http.StatusTooManyRequests, nil, "")
		return
	}
	defer rec.ReleaseExecutionSlot()

	res, err := send.SendText(r.Context(), s.httpClient, rec.ProviderData, input)
	if err != nil {
		s.handleOperationError(w, sessionID, rec, err, "SEND_FAILED", "Failed to send message to Facebook.")
		return
	}

	s.recordSessionActivity(sessionID)
	s.EventHub.Publish(map[string]any{"event": "message_sent", "sessionId": sessionID, "tenantId": rec.TenantID, "result": res})
	JSONSuccess(w, "Message sent successfully.", res, http.StatusOK)
}

func (s *Server) handleReactMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	tenantID := extractTenantID(r)
	rec, err := s.Registry.GetForTenant(sessionID, tenantID)
	if err != nil {
		if strings.Contains(err.Error(), "acceso denegado") {
			JSONError(w, "FORBIDDEN", "Cross-tenant access is not allowed.", http.StatusForbidden, nil, "")
			return
		}
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}
	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have active provider data.", http.StatusBadRequest, nil, "")
		return
	}

	input, err := DecodeJSONBody[types.ReactionInput](w, r, 64*1024)
	if err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid or exceeds allowed limits.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	if err := rec.AcquireExecutionSlot(r.Context()); err != nil {
		JSONError(w, "SESSION_EXECUTION_BLOCKED", err.Error(), http.StatusTooManyRequests, nil, "")
		return
	}
	defer rec.ReleaseExecutionSlot()

	res, err := reactions.ReactMessage(r.Context(), s.httpClient, rec.ProviderData, input.MessageID, input.Emoji, input.Action)
	if err != nil {
		s.handleOperationError(w, sessionID, rec, err, "REACTION_FAILED", "Failed to apply reaction.")
		return
	}

	s.recordSessionActivity(sessionID)
	s.EventHub.Publish(map[string]any{"event": "reaction_updated", "sessionId": sessionID, "tenantId": rec.TenantID, "messageId": input.MessageID, "action": input.Action})
	JSONSuccess(w, "Reaction applied successfully.", res, http.StatusOK)
}

func (s *Server) handleUnsendMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	tenantID := extractTenantID(r)
	rec, err := s.Registry.GetForTenant(sessionID, tenantID)
	if err != nil {
		if strings.Contains(err.Error(), "acceso denegado") {
			JSONError(w, "FORBIDDEN", "Cross-tenant access is not allowed.", http.StatusForbidden, nil, "")
			return
		}
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}
	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have active provider data.", http.StatusBadRequest, nil, "")
		return
	}

	input, err := DecodeJSONBody[types.UnsendInput](w, r, 64*1024)
	if err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid or exceeds allowed limits.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	if err := rec.AcquireExecutionSlot(r.Context()); err != nil {
		JSONError(w, "SESSION_EXECUTION_BLOCKED", err.Error(), http.StatusTooManyRequests, nil, "")
		return
	}
	defer rec.ReleaseExecutionSlot()

	res, err := unsend.UnsendMessage(r.Context(), s.httpClient, rec.ProviderData, input.MessageID)
	if err != nil {
		s.handleOperationError(w, sessionID, rec, err, "UNSEND_FAILED", "Failed to unsend message.")
		return
	}

	s.recordSessionActivity(sessionID)
	s.EventHub.Publish(map[string]any{"event": "message_unsent", "sessionId": sessionID, "tenantId": rec.TenantID, "messageId": input.MessageID})
	JSONSuccess(w, "Message unsend executed successfully.", res, http.StatusOK)
}

func (s *Server) handleSendMediaMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	tenantID := extractTenantID(r)
	rec, err := s.Registry.GetForTenant(sessionID, tenantID)
	if err != nil {
		if strings.Contains(err.Error(), "acceso denegado") {
			JSONError(w, "FORBIDDEN", "Cross-tenant access is not allowed.", http.StatusForbidden, nil, "")
			return
		}
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}
	if rec.ProviderData == nil {
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have active provider data.", http.StatusBadRequest, nil, "")
		return
	}

	input, err := DecodeJSONBody[types.MediaMessageInput](w, r, 64*1024)
	if err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid or exceeds allowed limits.", http.StatusUnprocessableEntity, nil, "")
		return
	}
	if len(input.MediaIDs) == 0 {
		JSONError(w, "INVALID_REQUEST", "At least one mediaId is required.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	if err := rec.AcquireExecutionSlot(r.Context()); err != nil {
		JSONError(w, "SESSION_EXECUTION_BLOCKED", err.Error(), http.StatusTooManyRequests, nil, "")
		return
	}
	defer rec.ReleaseExecutionSlot()

	// Cargar y adjuntar medios registrados
	var uploadedResults []*types.AttachmentResult
	for _, mID := range input.MediaIDs {
		filePath, err := s.MediaStore.GetFilePath(mID)
		if err != nil {
			JSONError(w, "MEDIA_NOT_FOUND", "Media file not found.", http.StatusBadRequest, map[string]any{"mediaId": mID}, "")
			return
		}
		f, err := os.Open(filePath)
		if err != nil {
			JSONError(w, "MEDIA_OPEN_FAILED", "Failed to open media file.", http.StatusInternalServerError, map[string]any{"mediaId": mID}, "")
			return
		}
		info, err := s.MediaStore.GetInfo(mID)
		if err != nil {
			_ = f.Close()
			JSONError(w, "MEDIA_NOT_FOUND", "Media metadata not found.", http.StatusBadRequest, map[string]any{"mediaId": mID}, "")
			return
		}
		uploaded, err := attachments.Upload(r.Context(), s.httpClient, rec.ProviderData, info.FileName, f, false)
		_ = f.Close()
		if err != nil || uploaded == nil {
			s.handleOperationError(w, sessionID, rec, err, "MEDIA_UPLOAD_FAILED", "Failed to upload media attachment.")
			return
		}
		uploadedResults = append(uploadedResults, uploaded)
	}

	msgInput := types.TextMessageInput{
		ThreadID:   input.ThreadID,
		Text:       input.Text,
		ThreadType: input.ThreadType,
	}
	res, err := send.SendWithAttachments(r.Context(), s.httpClient, rec.ProviderData, msgInput, uploadedResults)
	if err != nil {
		s.handleOperationError(w, sessionID, rec, err, "SEND_MEDIA_FAILED", "Failed to send media message.")
		return
	}

	s.recordSessionActivity(sessionID)
	s.EventHub.Publish(map[string]any{"event": "message_sent", "sessionId": sessionID, "tenantId": rec.TenantID, "result": res, "attachments": uploadedResults})
	JSONSuccess(w, "Media message sent successfully.", map[string]any{
		"result":      res,
		"attachments": uploadedResults,
	}, http.StatusOK)
}
