package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bradez-rd/fbreego/pkg/core/auth"
	"github.com/bradez-rd/fbreego/pkg/core/lightspeed"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/features/thread"
	"github.com/bradez-rd/fbreego/pkg/messaging/listener"
)

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	tenantID := extractTenantID(r)
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
	input, err := DecodeJSONBody[types.SessionCreateInput](w, r, 256*1024)
	if err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid or exceeds allowed limits.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	callerTenant := extractTenantID(r)
	if input.TenantID == "" {
		input.TenantID = callerTenant
	} else if callerTenant != "" && input.TenantID != callerTenant {
		JSONError(w, "FORBIDDEN", "Tenant mismatch between request context and payload.", http.StatusForbidden, nil, "")
		return
	}

	// Si no se proporcionaron cookies pero sí credenciales, realizar login primero
	if len(input.Cookies) == 0 && input.Email != "" && input.Password != "" {
		payload, err := auth.FB4ALogin(r.Context(), s.httpClient, input.Email, input.Password, input.TOTPKey)
		if err != nil {
			if challengeErr, ok := err.(*types.TwoFactorChallengeError); ok {
				JSONResponse(w, http.StatusForbidden, types.ServiceResponse{
					Code:    "2FA_CHALLENGE_REQUIRED",
					Message: "Facebook requiere completar la verificación 2FA / checkpoint.",
					Results: map[string]any{
						"status":       "challenge_required",
						"code":         challengeErr.Code,
						"subcode":      challengeErr.Subcode,
						"title":        challengeErr.Title,
						"description":  challengeErr.Description,
						"challengeUrl": challengeErr.ChallengeURL,
						"totpCode":     challengeErr.TOTPCode,
						"fbtraceId":    challengeErr.FBTraceID,
					},
				})
				return
			}
			JSONError(w, "LOGIN_FAILED", err.Error(), http.StatusBadRequest, nil, "")
			return
		}

		var cookies []types.CookieInput
		for _, item := range payload.CookiesKeyValueList {
			name, _ := item["name"].(string)
			value, _ := item["value"].(string)
			if name != "" && value != "" {
				cookies = append(cookies, types.CookieInput{Name: name, Value: value})
			}
		}
		input.Cookies = cookies
		if input.Label == "" {
			input.Label = input.Email
		}
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

func (s *Server) handleLoginSession(w http.ResponseWriter, r *http.Request) {
	var input types.LoginCredentialsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid.", http.StatusUnprocessableEntity, nil, "")
		return
	}
	identifier := strings.TrimSpace(input.Email)
	if identifier == "" {
		identifier = strings.TrimSpace(input.Username)
	}
	if identifier == "" || strings.TrimSpace(input.Password) == "" {
		JSONError(w, "INVALID_CREDENTIALS", "Usuario/correo y contraseña son requeridos.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	payload, err := auth.FB4ALogin(r.Context(), s.httpClient, identifier, input.Password, input.TOTPKey)
	if err != nil {
		if challengeErr, ok := err.(*types.TwoFactorChallengeError); ok {
			JSONResponse(w, http.StatusForbidden, types.ServiceResponse{
				Code:    "2FA_CHALLENGE_REQUIRED",
				Message: "Facebook requiere verificación 2FA / checkpoint.",
				Results: map[string]any{
					"status":       "challenge_required",
					"code":         challengeErr.Code,
					"subcode":      challengeErr.Subcode,
					"title":        challengeErr.Title,
					"description":  challengeErr.Description,
					"challengeUrl": challengeErr.ChallengeURL,
					"totpCode":     challengeErr.TOTPCode,
					"fbtraceId":    challengeErr.FBTraceID,
				},
			})
			return
		}
		JSONError(w, "LOGIN_FAILED", err.Error(), http.StatusBadRequest, nil, "")
		return
	}

	var cookies []types.CookieInput
	for _, item := range payload.CookiesKeyValueList {
		name, _ := item["name"].(string)
		value, _ := item["value"].(string)
		if name != "" && value != "" {
			cookies = append(cookies, types.CookieInput{Name: name, Value: value})
		}
	}

	label := input.Label
	if label == "" {
		label = identifier
	}

	createInput := types.SessionCreateInput{
		Label:    label,
		TenantID: input.TenantID,
		Cookies:  cookies,
	}

	rec, err := s.Registry.Create(r.Context(), createInput)
	if err != nil {
		JSONError(w, "SESSION_VALIDATION_FAILED", err.Error(), http.StatusBadRequest, nil, "")
		return
	}

	s.RecordAudit("session_login", fmt.Sprintf("Session %s created via login", rec.SessionID), rec.SessionID)
	s.EventHub.EmitSessionStatus(rec.SessionID, rec.Status, "")
	pub := rec.Public()
	pub["listenerStatus"] = "idle"
	JSONSuccess(w, "Session authenticated and created successfully.", pub, http.StatusCreated)
}

// handleCredentialsLogin implementa la API de login con usuario/contraseña compatible con el adaptador fbreego de SocialManager.
func (s *Server) handleCredentialsLogin(w http.ResponseWriter, r *http.Request) {
	var input types.CredentialAuthInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	username := strings.TrimSpace(input.Username)
	if username == "" {
		username = strings.TrimSpace(input.Email)
	}
	password := input.Password
	if username == "" || strings.TrimSpace(password) == "" {
		JSONError(w, "INVALID_CREDENTIALS", "Usuario/correo y contraseña son requeridos.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	payload, err := auth.FB4ALogin(r.Context(), s.httpClient, username, password, input.TOTPKey)
	if err != nil {
		if challengeErr, ok := err.(*types.TwoFactorChallengeError); ok {
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			challengeID := "chal_" + base64.RawURLEncoding.EncodeToString(b)
			expiresAt := time.Now().Add(10 * time.Minute)

			s.challengesMu.Lock()
			s.challenges[challengeID] = Pending2FAChallenge{
				ChallengeID:  challengeID,
				Username:     username,
				Password:     password,
				Label:        input.Label,
				TenantID:     input.TenantID,
				ExpiresAt:    expiresAt,
				ChallengeURL: challengeErr.ChallengeURL,
			}
			s.challengesMu.Unlock()

			// Respuesta estandarizada esperada por el adaptador de SocialManager y frontend
			JSONSuccess(w, "Facebook requiere código 2FA.", map[string]any{
				"status":       "2FA_REQUIRED",
				"challengeId":  challengeID,
				"expiresAt":    expiresAt.Format(time.RFC3339),
				"challengeUrl": challengeErr.ChallengeURL,
				"totpCode":     challengeErr.TOTPCode,
			}, http.StatusOK)
			return
		}

		JSONError(w, "LOGIN_FAILED", err.Error(), http.StatusBadRequest, nil, "")
		return
	}

	var cookies []types.CookieInput
	for _, item := range payload.CookiesKeyValueList {
		name, _ := item["name"].(string)
		value, _ := item["value"].(string)
		if name != "" && value != "" {
			cookies = append(cookies, types.CookieInput{Name: name, Value: value})
		}
	}

	label := input.Label
	if label == "" {
		label = username
	}

	createInput := types.SessionCreateInput{
		Label:    label,
		TenantID: input.TenantID,
		Cookies:  cookies,
	}

	rec, err := s.Registry.Create(r.Context(), createInput)
	if err != nil {
		JSONError(w, "SESSION_VALIDATION_FAILED", err.Error(), http.StatusBadRequest, nil, "")
		return
	}

	s.RecordAudit("session_login", fmt.Sprintf("Session %s created via credentials", rec.SessionID), rec.SessionID)
	s.EventHub.EmitSessionStatus(rec.SessionID, rec.Status, "")
	pub := rec.Public()
	pub["listenerStatus"] = "idle"
	JSONSuccess(w, "Session authenticated and created successfully.", pub, http.StatusOK)
}

// handleCompleteTwoFactor verifica el código 2FA ingresado por el usuario y finaliza la creación de la sesión.
func (s *Server) handleCompleteTwoFactor(w http.ResponseWriter, r *http.Request) {
	var input types.TwoFactorAuthInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	challengeID := strings.TrimSpace(input.ChallengeID)
	code := strings.TrimSpace(input.Code)
	if challengeID == "" || code == "" {
		JSONError(w, "INVALID_INPUT", "challengeId y code son requeridos.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	s.challengesMu.Lock()
	chal, exists := s.challenges[challengeID]
	if exists {
		delete(s.challenges, challengeID)
	}
	s.challengesMu.Unlock()

	if !exists || time.Now().After(chal.ExpiresAt) {
		JSONError(w, "CHALLENGE_EXPIRED", "El desafío 2FA no existe o ha expirado.", http.StatusGone, nil, "")
		return
	}

	payload, err := auth.FB4ALogin(r.Context(), s.httpClient, chal.Username, chal.Password, code)
	if err != nil {
		JSONError(w, "2FA_VERIFICATION_FAILED", fmt.Sprintf("Error al verificar código 2FA: %v", err), http.StatusBadRequest, nil, "")
		return
	}

	var cookies []types.CookieInput
	for _, item := range payload.CookiesKeyValueList {
		name, _ := item["name"].(string)
		value, _ := item["value"].(string)
		if name != "" && value != "" {
			cookies = append(cookies, types.CookieInput{Name: name, Value: value})
		}
	}

	label := chal.Label
	if label == "" {
		label = chal.Username
	}

	createInput := types.SessionCreateInput{
		Label:    label,
		TenantID: chal.TenantID,
		Cookies:  cookies,
	}

	rec, err := s.Registry.Create(r.Context(), createInput)
	if err != nil {
		JSONError(w, "SESSION_VALIDATION_FAILED", err.Error(), http.StatusBadRequest, nil, "")
		return
	}

	s.RecordAudit("session_2fa_complete", fmt.Sprintf("Session %s created after 2FA verification", rec.SessionID), rec.SessionID)
	s.EventHub.EmitSessionStatus(rec.SessionID, rec.Status, "")
	pub := rec.Public()
	pub["listenerStatus"] = "idle"
	JSONSuccess(w, "Session authenticated and created successfully after 2FA.", pub, http.StatusOK)
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var input types.LoginCredentialsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid.", http.StatusUnprocessableEntity, nil, "")
		return
	}
	if strings.TrimSpace(input.Email) == "" || strings.TrimSpace(input.Password) == "" {
		JSONError(w, "INVALID_CREDENTIALS", "Email y password son requeridos.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	payload, err := auth.FB4ALogin(r.Context(), s.httpClient, input.Email, input.Password, input.TOTPKey)
	if err != nil {
		if challengeErr, ok := err.(*types.TwoFactorChallengeError); ok {
			JSONResponse(w, http.StatusForbidden, types.ServiceResponse{
				Code:    "2FA_CHALLENGE_REQUIRED",
				Message: "Facebook requiere verificación 2FA / checkpoint.",
				Results: map[string]any{
					"status":       "challenge_required",
					"code":         challengeErr.Code,
					"subcode":      challengeErr.Subcode,
					"title":        challengeErr.Title,
					"description":  challengeErr.Description,
					"challengeUrl": challengeErr.ChallengeURL,
					"totpCode":     challengeErr.TOTPCode,
					"fbtraceId":    challengeErr.FBTraceID,
				},
			})
			return
		}
		JSONError(w, "LOGIN_FAILED", err.Error(), http.StatusBadRequest, nil, "")
		return
	}

	JSONSuccess(w, "FB4A authentication successful.", payload, http.StatusOK)
}

func (s *Server) handleGenerateTOTP(w http.ResponseWriter, r *http.Request) {
	var input types.TOTPGenerateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "The request payload is invalid.", http.StatusUnprocessableEntity, nil, "")
		return
	}
	if strings.TrimSpace(input.Key) == "" {
		JSONError(w, "INVALID_KEY", "La clave TOTP es requerida.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	code, remaining, err := auth.CurrentTOTP(input.Key)
	if err != nil {
		JSONError(w, "TOTP_GENERATION_FAILED", err.Error(), http.StatusBadRequest, nil, "")
		return
	}

	JSONSuccess(w, "TOTP code generated successfully.", types.TOTPResponse{
		Code:             code,
		RemainingSeconds: remaining,
	}, http.StatusOK)
}

func (s *Server) handleGetSessionStatus(w http.ResponseWriter, r *http.Request) {
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
	tenantID := extractTenantID(r)
	if _, err := s.Registry.GetForTenant(sessionID, tenantID); err != nil {
		if strings.Contains(err.Error(), "acceso denegado") {
			JSONError(w, "FORBIDDEN", "Cross-tenant access is not allowed.", http.StatusForbidden, nil, "")
			return
		}
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}

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
	tenantID := extractTenantID(r)
	s.ListenerManager.StopListening(sessionID)
	err := s.Registry.DeleteForTenant(sessionID, tenantID)
	if err != nil {
		if strings.Contains(err.Error(), "acceso denegado") {
			JSONError(w, "FORBIDDEN", "Cross-tenant access is not allowed.", http.StatusForbidden, nil, "")
			return
		}
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}

	s.RecordAudit("session_delete", "Session deleted", sessionID)
	s.EventHub.EmitSessionStatus(sessionID, "deleted", "")
	JSONSuccess(w, "Session deleted successfully.", map[string]string{"sessionId": sessionID}, http.StatusOK)
}

func (s *Server) handleStartListening(w http.ResponseWriter, r *http.Request) {
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

	if s.ListenerManager == nil {
		JSONError(w, "LISTENER_UNAVAILABLE", "Listener manager is not initialized.", http.StatusServiceUnavailable, nil, "")
		return
	}

	mqttListener := listener.NewMQTTListener(rec.ProviderData)

	worker := func(ctx context.Context) {
		if err := mqttListener.Start(ctx); err != nil {
			s.RecordAudit("listener_error", err.Error(), sessionID)
			return
		}
		defer mqttListener.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-mqttListener.Messages():
				if !ok {
					return
				}
				s.Registry.RecordActivity(sessionID)
				eventName := string(msg.EventType)
				if eventName == "" {
					eventName = string(listener.MessageReceived)
				}

				eventPayload := map[string]any{
					"sessionId":  sessionID,
					"threadId":   msg.ThreadID,
					"messageId":  msg.MessageID,
					"senderId":   msg.SenderID,
					"text":       msg.Body,
					"timestamp":  msg.TimestampMs,
					"isGroup":    msg.IsGroup,
					"threadType": msg.ThreadType,
				}

				if s.EventHub != nil {
					envelope := map[string]any{
						"event":     eventName,
						"sessionId": sessionID,
						"data":      eventPayload,
					}
					if msg.TimestampMs > 0 {
						envelope["timestamp"] = time.UnixMilli(msg.TimestampMs).UTC().Format(time.RFC3339Nano)
					}
					s.EventHub.Publish(envelope)
				} else if s.Dispatcher != nil {
					s.Dispatcher.Enqueue(eventName, eventPayload)
				}
			}
		}
	}

	if err := s.ListenerManager.StartListening(context.Background(), sessionID, worker); err != nil {
		JSONError(w, "LISTENER_START_FAILED", err.Error(), http.StatusBadRequest, nil, "")
		return
	}

	s.RecordAudit("listener_start", "MQTT listener started", sessionID)
	JSONSuccess(w, "Session listener started successfully.", map[string]any{
		"sessionId": sessionID,
		"status":    "listening",
	}, http.StatusOK)
}

func (s *Server) handleStopListening(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	tenantID := extractTenantID(r)
	if _, err := s.Registry.GetForTenant(sessionID, tenantID); err != nil {
		if strings.Contains(err.Error(), "acceso denegado") {
			JSONError(w, "FORBIDDEN", "Cross-tenant access is not allowed.", http.StatusForbidden, nil, "")
			return
		}
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}

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
	tenantID := extractTenantID(r)
	if _, err := s.Registry.GetForTenant(sessionID, tenantID); err != nil {
		if strings.Contains(err.Error(), "acceso denegado") {
			JSONError(w, "FORBIDDEN", "Cross-tenant access is not allowed.", http.StatusForbidden, nil, "")
			return
		}
		JSONError(w, "SESSION_NOT_FOUND", "Session not found.", http.StatusNotFound, nil, "")
		return
	}

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
		JSONError(w, "SESSION_NOT_VALIDATED", "Session does not have provider data.", http.StatusBadRequest, nil, "")
		return
	}

	threadsData, err := thread.FetchAllThreads(r.Context(), s.httpClient, rec.ProviderData)
	if err == nil {
		items := thread.ExtractThreadItems(threadsData)
		if len(items) > 0 {
			JSONSuccess(w, "Threads retrieved successfully.", map[string]any{
				"items": items,
				"count": len(items),
			}, http.StatusOK)
			return
		}
	}

	// Fallback con LightSpeed HTML parser
	lsData, fallbackErr := lightspeed.FetchLightSpeedData(r.Context(), s.httpClient, rec.BuildCookieHeader())
	if fallbackErr != nil && err != nil {
		s.RecordAudit("thread_fetch_failed", fmt.Sprintf("primary: %v; fallback: %v", err, fallbackErr), sessionID)
		JSONError(w, "THREADS_FETCH_FAILED", "Unable to retrieve threads from Facebook.", http.StatusBadGateway, nil, "")
		return
	}
	var fallbackItems []map[string]any
	for _, t := range lsData {
		fallbackItems = append(fallbackItems, map[string]any{
			"threadId":    t.ThreadID,
			"name":        t.Name,
			"type":        "user",
			"unreadCount": t.UnreadCount,
		})
	}
	JSONSuccess(w, "Threads retrieved via LightSpeed fallback.", map[string]any{
		"items": fallbackItems,
		"count": len(fallbackItems),
	}, http.StatusOK)
}
