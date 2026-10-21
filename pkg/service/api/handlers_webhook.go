package api

import (
	"encoding/json"
	"net/http"

	"github.com/bradez-rd/fbreego/pkg/core/types"
)

func (s *Server) handleGetWebhookConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.WebhookStore.GetPublic()
	JSONSuccess(w, "Webhook configuration retrieved.", cfg, http.StatusOK)
}

func (s *Server) handleUpdateWebhookConfig(w http.ResponseWriter, r *http.Request) {
	var input types.WebhookConfigInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	updated, err := s.WebhookStore.Update(input.Enabled, input.URL, input.Events, input.Secret, input.ClearSecret)
	if err != nil {
		JSONError(w, "WEBHOOK_CONFIG_INVALID", err.Error(), http.StatusBadRequest, nil, "")
		return
	}

	s.RecordAudit("webhook_update", "Webhook configuration updated", "")
	JSONSuccess(w, "Webhook configuration updated successfully.", updated, http.StatusOK)
}

func (s *Server) handleClearWebhookConfig(w http.ResponseWriter, r *http.Request) {
	cleared := s.WebhookStore.Clear()
	s.RecordAudit("webhook_clear", "Webhook configuration cleared", "")
	JSONSuccess(w, "Webhook configuration cleared.", cleared, http.StatusOK)
}

func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	var input types.WebhookTestInput
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}

	targetURL := ""
	if input.URL != nil {
		targetURL = *input.URL
	}
	secret := ""
	if input.Secret != nil {
		secret = *input.Secret
	}

	result, err := s.Dispatcher.TestProbe(r.Context(), targetURL, secret)
	if err != nil {
		JSONError(w, "WEBHOOK_PROBE_FAILED", err.Error(), http.StatusBadRequest, nil, "")
		return
	}

	JSONSuccess(w, "Webhook probe completed.", result, http.StatusOK)
}
