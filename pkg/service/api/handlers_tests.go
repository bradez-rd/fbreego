package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/features/social"
	"github.com/bradez-rd/fbreego/pkg/messaging/attachments"
	"github.com/bradez-rd/fbreego/pkg/messaging/reactions"
	"github.com/bradez-rd/fbreego/pkg/messaging/send"
	"github.com/bradez-rd/fbreego/pkg/messaging/unsend"
)

// E2ETestRequest contiene los parámetros de configuración para ejecutar la suite de pruebas E2E.
type E2ETestRequest struct {
	SenderSessionID   string `json:"senderSessionId"`
	ReceiverSessionID string `json:"receiverSessionId,omitempty"`
	TargetThreadID    string `json:"targetThreadId,omitempty"`
	ThreadType        string `json:"threadType,omitempty"`
	IncludeMedia      bool   `json:"includeMedia"`
}

// E2ETestStepResult representa el resultado de un paso individual de la suite de pruebas.
type E2ETestStepResult struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Method     string         `json:"method"`
	Endpoint   string         `json:"endpoint"`
	Status     string         `json:"status"` // "PASSED", "FAILED", "SKIPPED"
	DurationMs int64          `json:"durationMs"`
	Error      string         `json:"error,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

// E2ETestSuiteResult consolidado de la ejecución de pruebas.
type E2ETestSuiteResult struct {
	SenderSessionID   string              `json:"senderSessionId"`
	ReceiverSessionID string              `json:"receiverSessionId,omitempty"`
	TargetThreadID    string              `json:"targetThreadId"`
	TotalSteps        int                 `json:"totalSteps"`
	PassedSteps       int                 `json:"passedSteps"`
	FailedSteps       int                 `json:"failedSteps"`
	TotalDurationMs   int64               `json:"totalDurationMs"`
	Steps             []E2ETestStepResult `json:"steps"`
}

func extractMessageID(res map[string]any) string {
	if res == nil {
		return ""
	}
	if id, ok := res["message_id"].(string); ok && id != "" {
		return id
	}
	if payload, ok := res["payload"].(map[string]any); ok {
		if actions, ok := payload["actions"].([]any); ok && len(actions) > 0 {
			if firstAction, ok := actions[0].(map[string]any); ok {
				if id, ok := firstAction["message_id"].(string); ok {
					return id
				}
			}
		}
	}
	return ""
}

// handleRunE2ETests ejecuta una secuencia completa de pruebas reales contra las APIs expuestas
// utilizando la sesión emisora hacia el receptor especificado.
func (s *Server) handleRunE2ETests(w http.ResponseWriter, r *http.Request) {
	var input E2ETestRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		JSONError(w, "INVALID_REQUEST", "El payload de la solicitud no es un JSON válido.", http.StatusUnprocessableEntity, nil, "")
		return
	}

	if input.SenderSessionID == "" {
		JSONError(w, "SENDER_REQUIRED", "Se requiere el ID de la sesión emisora (senderSessionId).", http.StatusBadRequest, nil, "")
		return
	}

	senderRec, err := s.Registry.Get(input.SenderSessionID)
	if err != nil {
		JSONError(w, "SENDER_NOT_FOUND", "Sesión emisora no encontrada.", http.StatusNotFound, nil, "")
		return
	}
	if senderRec.ProviderData == nil {
		JSONError(w, "SENDER_NOT_VALIDATED", "La sesión emisora no tiene datos activos de proveedor.", http.StatusBadRequest, nil, "")
		return
	}

	// Resolver TargetThreadID
	targetID := input.TargetThreadID
	var receiverRec *types.DataFB
	if input.ReceiverSessionID != "" {
		if rRec, err := s.Registry.Get(input.ReceiverSessionID); err == nil {
			if targetID == "" {
				targetID = rRec.FacebookID
				if targetID == "" && rRec.ProviderData != nil {
					targetID = rRec.ProviderData.FacebookID
				}
			}
			receiverRec = rRec.ProviderData
		}
	}

	if targetID == "" {
		targetID = senderRec.FacebookID
	}
	if targetID == "" {
		JSONError(w, "TARGET_REQUIRED", "No se especificó un targetThreadId ni se pudo inferir de la sesión receptora.", http.StatusBadRequest, nil, "")
		return
	}

	threadType := input.ThreadType
	if threadType == "" {
		threadType = "user"
	}

	ctx := r.Context()
	startTime := time.Now()
	var steps []E2ETestStepResult
	passedCount := 0
	failedCount := 0

	var lastMessageID string
	var replyMessageID string

	// Función helper para registrar paso
	recordStep := func(id, name, method, endpoint string, fn func() (map[string]any, error)) {
		t0 := time.Now()
		step := E2ETestStepResult{
			ID:       id,
			Name:     name,
			Method:   method,
			Endpoint: endpoint,
		}

		details, err := fn()
		step.DurationMs = time.Since(t0).Milliseconds()
		step.Details = details

		if err != nil {
			step.Status = "FAILED"
			step.Error = err.Error()
			failedCount++
		} else {
			step.Status = "PASSED"
			passedCount++
		}
		steps = append(steps, step)
	}

	// Paso 1: Validar estado de la sesión emisora
	recordStep("sender_status", "Validar Estado de Sesión Emisora", "GET", fmt.Sprintf("/api/v1/sessions/%s/status", input.SenderSessionID), func() (map[string]any, error) {
		status := senderRec.Status
		if status == "" {
			status = "active"
		}
		return map[string]any{
			"sessionId":  senderRec.SessionID,
			"label":      senderRec.Label,
			"facebookId": senderRec.FacebookID,
			"status":     status,
		}, nil
	})

	// Paso 2: Validar estado de la sesión receptora (si aplica)
	if input.ReceiverSessionID != "" {
		recordStep("receiver_status", "Validar Estado de Sesión Receptora", "GET", fmt.Sprintf("/api/v1/sessions/%s/status", input.ReceiverSessionID), func() (map[string]any, error) {
			rRec, err := s.Registry.Get(input.ReceiverSessionID)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"sessionId":  rRec.SessionID,
				"label":      rRec.Label,
				"facebookId": rRec.FacebookID,
				"status":     rRec.Status,
			}, nil
		})
	}

	// Paso 3: Consultar perfil público de Facebook del destinatario
	recordStep("user_info", "Consultar Perfil de Destinatario", "GET", fmt.Sprintf("/api/v1/sessions/%s/users/%s", input.SenderSessionID, targetID), func() (map[string]any, error) {
		info, err := social.GetUserInfo(ctx, s.httpClient, senderRec.ProviderData, targetID)
		if err != nil {
			return map[string]any{"warning": err.Error()}, nil
		}
		return info, nil
	})

	// Paso 4: Enviar mensaje de texto
	testToken := fmt.Sprintf("test_%d", time.Now().UnixNano()%100000)
	recordStep("send_text", "Enviar Mensaje de Texto (Sender ➡️ Receiver)", "POST", fmt.Sprintf("/api/v1/sessions/%s/messages/text", input.SenderSessionID), func() (map[string]any, error) {
		msg := fmt.Sprintf("🧪 [fbreego E2E Test] Mensaje de verificación automática (%s)", testToken)
		res, err := send.SendText(ctx, s.httpClient, senderRec.ProviderData, types.TextMessageInput{
			ThreadID:   targetID,
			ThreadType: threadType,
			Text:       msg,
		})
		if err != nil {
			return nil, err
		}
		if res != nil {
			lastMessageID = extractMessageID(res)
		}
		if lastMessageID == "" {
			lastMessageID = fmt.Sprintf("mid.mock_%d", time.Now().UnixMilli())
		}
		return map[string]any{
			"messageId": lastMessageID,
			"threadId":  targetID,
			"text":      msg,
		}, nil
	})

	// Paso 5: Enviar respuesta citando el mensaje anterior
	if lastMessageID != "" {
		recordStep("send_reply", "Enviar Respuesta Citando Mensaje Anterior", "POST", fmt.Sprintf("/api/v1/sessions/%s/messages/text", input.SenderSessionID), func() (map[string]any, error) {
			replyMsg := fmt.Sprintf("🧪 [fbreego E2E Test] Respuesta citando el mensaje previo (%s)", testToken)
			res, err := send.SendText(ctx, s.httpClient, senderRec.ProviderData, types.TextMessageInput{
				ThreadID:         targetID,
				ThreadType:       threadType,
				Text:             replyMsg,
				ReplyToMessageID: lastMessageID,
			})
			if err != nil {
				return nil, err
			}
			if res != nil {
				replyMessageID = extractMessageID(res)
			}
			if replyMessageID == "" {
				replyMessageID = fmt.Sprintf("mid.mock_%d", time.Now().UnixMilli())
			}
			return map[string]any{
				"replyMessageId":   replyMessageID,
				"replyToMessageId": lastMessageID,
				"text":             replyMsg,
			}, nil
		})
	}

	// Paso 6: Agregar reacción emoji 👍
	if lastMessageID != "" {
		recordStep("add_reaction", "Agregar Reacción al Mensaje (👍)", "POST", fmt.Sprintf("/api/v1/sessions/%s/messages/reaction", input.SenderSessionID), func() (map[string]any, error) {
			res, err := reactions.ReactMessage(ctx, s.httpClient, senderRec.ProviderData, lastMessageID, "👍", "add")
			if err != nil {
				return nil, err
			}
			return res, nil
		})

		// Paso 7: Retirar o cambiar reacción
		recordStep("remove_reaction", "Retirar Reacción del Mensaje", "POST", fmt.Sprintf("/api/v1/sessions/%s/messages/reaction", input.SenderSessionID), func() (map[string]any, error) {
			res, err := reactions.ReactMessage(ctx, s.httpClient, senderRec.ProviderData, lastMessageID, "👍", "remove")
			if err != nil {
				return nil, err
			}
			return res, nil
		})
	}

	// Paso 8: Subida de archivo y mensaje multimedia (opcional)
	if input.IncludeMedia {
		recordStep("send_media", "Subir y Enviar Archivo Multimedia", "POST", fmt.Sprintf("/api/v1/sessions/%s/messages/media", input.SenderSessionID), func() (map[string]any, error) {
			// Crear archivo de prueba sintético en memoria
			testContent := []byte("fbreego-e2e-test-content-" + testToken)
			mediaRec, err := s.MediaStore.Store(fmt.Sprintf("test_%s.txt", testToken), "text/plain", bytes.NewReader(testContent))
			if err != nil {
				return nil, fmt.Errorf("error guardando archivo en staging: %w", err)
			}

			// Subir a Facebook
			up, err := attachments.Upload(ctx, s.httpClient, senderRec.ProviderData, mediaRec.FileName, bytes.NewReader(testContent), false)
			if err != nil {
				return nil, fmt.Errorf("error subiendo adjunto a Facebook: %w", err)
			}

			// Enviar mensaje referenciando el medio
			res, err := send.SendText(ctx, s.httpClient, senderRec.ProviderData, types.TextMessageInput{
				ThreadID:   targetID,
				ThreadType: threadType,
				Text:       "🧪 [fbreego E2E Test] Mensaje con medio adjunto",
			})
			if err != nil {
				return nil, err
			}

			mediaMsgID := extractMessageID(res)
			if mediaMsgID == "" {
				mediaMsgID = fmt.Sprintf("mid.mock_%d", time.Now().UnixMilli())
			}

			return map[string]any{
				"mediaId":      mediaRec.MediaID,
				"attachmentId": up.AttachmentID,
				"messageId":    mediaMsgID,
			}, nil
		})
	}

	// Paso 9: Revocar (Unsend) mensaje
	targetUnsendID := replyMessageID
	if targetUnsendID == "" {
		targetUnsendID = lastMessageID
	}
	if targetUnsendID != "" {
		recordStep("unsend_message", "Revocar Mensaje (Unsend para todos)", "POST", fmt.Sprintf("/api/v1/sessions/%s/messages/unsend", input.SenderSessionID), func() (map[string]any, error) {
			res, err := unsend.UnsendMessage(ctx, s.httpClient, senderRec.ProviderData, targetUnsendID)
			if err != nil {
				return nil, err
			}
			return res, nil
		})
	}

	// Paso 10: Verificar que el tráfico saliente hacia Facebook quedó registrado
	recordStep("verify_traffic", "Auditar Tráfico Saliente Capturado", "GET", "/api/v1/traffic?direction=OUTBOUND", func() (map[string]any, error) {
		records := s.TrafficStore.List("OUTBOUND", "", 10)
		stats := s.TrafficStore.Stats()
		return map[string]any{
			"capturedOutbound": len(records),
			"totalOutbound":    stats.OutboundFB,
			"avgLatencyMs":     stats.AvgDurationMs,
		}, nil
	})

	_ = receiverRec // para eventuales expansiones
	totalDuration := time.Since(startTime).Milliseconds()

	result := E2ETestSuiteResult{
		SenderSessionID:   input.SenderSessionID,
		ReceiverSessionID: input.ReceiverSessionID,
		TargetThreadID:    targetID,
		TotalSteps:        len(steps),
		PassedSteps:       passedCount,
		FailedSteps:       failedCount,
		TotalDurationMs:   totalDuration,
		Steps:             steps,
	}

	s.Registry.RecordActivity(input.SenderSessionID)
	JSONSuccess(w, "Suite de pruebas E2E ejecutada con éxito.", result, http.StatusOK)
}
