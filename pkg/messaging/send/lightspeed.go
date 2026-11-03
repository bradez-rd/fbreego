package send

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

const (
	GraphQLURL             = "https://www.facebook.com/api/graphql/"
	LightSpeedRequestDocID = "9697184873702141" // LSPlatformGraphQLLightspeedRequestQuery
	LightSpeedVersionID    = "39520395834272091"
)

// ErrCutoverToE2EE indica que Meta rechazó el envío en texto abierto porque la conversación ha sido migrada obligatoriamente a E2EE.
var ErrCutoverToE2EE = errors.New("el hilo requiere cifrado de extremo a extremo (E2EE / Labyrinth)")

// SendMessageTaskPayload representa los campos de la tarea LightSpeed 46 (SendMessageTask).
type SendMessageTaskPayload struct {
	ThreadID                 int64   `json:"thread_id"`
	Otid                     string  `json:"otid"`
	Source                   int     `json:"source"`
	SendType                 int     `json:"send_type"`
	SyncGroup                int     `json:"sync_group"`
	Text                     string  `json:"text,omitempty"`
	InitiatingSource         int     `json:"initiating_source"`
	SkipUrlPreviewGen        int     `json:"skip_url_preview_gen"`
	TextHasLinks             int     `json:"text_has_links"`
	MultitabEnv              int     `json:"multitab_env"`
	AttachmentFBIds          []int64 `json:"attachment_fbids,omitempty"`
	ReplySourceID            string  `json:"reply_source_id,omitempty"`
}

// SendLightSpeedOpen envía un mensaje a un hilo abierto (Grupo, Marketplace, o Página) usando el protocolo moderno LightSpeed.
func SendLightSpeedOpen(ctx context.Context, client *http.Client, fb *types.DataFB, input types.TextMessageInput, attachmentIDs []int64) (map[string]any, error) {
	if fb == nil {
		return nil, fmt.Errorf("datos de sesión no configurados")
	}
	if strings.TrimSpace(input.ThreadID) == "" {
		return nil, fmt.Errorf("threadId es obligatorio")
	}
	if client == nil {
		client = http.DefaultClient
	}

	threadIDInt, err := strconv.ParseInt(strings.TrimSpace(input.ThreadID), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("threadId inválido para LightSpeed: %w", err)
	}

	otid := utils.GenThreadingID()
	task := SendMessageTaskPayload{
		ThreadID:          threadIDInt,
		Otid:              otid,
		Source:            0,
		SendType:          1,
		SyncGroup:         1,
		Text:              input.Text,
		InitiatingSource:  1, // FACEBOOK_INBOX
		SkipUrlPreviewGen: 0,
		TextHasLinks:      0,
		MultitabEnv:       0,
		AttachmentFBIds:   attachmentIDs,
		ReplySourceID:     input.ReplyToMessageID,
	}

	taskMarshalled, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("error al serializar SendMessageTask: %w", err)
	}

	taskData := map[string]any{
		"failure_count": nil,
		"label":         "46", // SendMessageTask
		"payload":       string(taskMarshalled),
		"queue_name":    strconv.FormatInt(threadIDInt, 10),
		"task_id":       0,
	}

	taskPayload := map[string]any{
		"epoch_id":      time.Now().UnixMilli(),
		"tasks":         []any{taskData},
		"data_trace_id": "",
		"version_id":    LightSpeedVersionID,
	}

	taskPayloadMarshalled, err := json.Marshal(taskPayload)
	if err != nil {
		return nil, fmt.Errorf("error al serializar payload de tareas LightSpeed: %w", err)
	}

	deviceID := uuid.NewString()
	lsVariables := map[string]any{
		"deviceId":              deviceID,
		"includeChatVisibility": false,
		"requestId":             0,
		"requestPayload":        string(taskPayloadMarshalled),
		"requestType":           3,
	}

	variablesJSON, err := json.Marshal(lsVariables)
	if err != nil {
		return nil, fmt.Errorf("error al serializar variables GraphQL: %w", err)
	}

	form := url.Values{
		"fb_dtsg":   {fb.FbDtsg},
		"jazoest":   {fb.Jazoest},
		"doc_id":    {LightSpeedRequestDocID},
		"variables": {string(variablesJSON)},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", GraphQLURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("error al construir petición LightSpeed: %w", err)
	}

	headers := utils.DefaultHeaders("www.facebook.com")
	for k, v := range headers {
		req.Header[k] = v
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fb.CookieFacebook)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fallo de red al invocar LightSpeed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error al leer respuesta de LightSpeed: %w", err)
	}

	bodyStr := string(bodyBytes)

	// Detección de corte obligatorio a E2EE
	if strings.Contains(bodyStr, "LSCutoverHandleInvalidSendToOpen") {
		return nil, ErrCutoverToE2EE
	}

	var parsedResp map[string]any
	if err := json.Unmarshal(bodyBytes, &parsedResp); err != nil {
		return nil, fmt.Errorf("respuesta de LightSpeed no es JSON válido (HTTP %d, body: %q): %w", resp.StatusCode, bodyStr, err)
	}

	if errVal, ok := parsedResp["errors"]; ok {
		return nil, fmt.Errorf("error GraphQL en LightSpeed: %v", errVal)
	}

	// Extraer message_id asignado si está disponible en la respuesta
	messageID := otid
	if strings.Contains(bodyStr, "LSReplaceOptimsiticMessage") || strings.Contains(bodyStr, "LSUpdateThreadSnippet") {
		// Envío confirmado y procesado por LightSpeed
		return map[string]any{
			"status":    "delivered",
			"protocol":  "lightspeed_open",
			"messageId": messageID,
			"threadId":  input.ThreadID,
			"otid":      otid,
			"raw":       parsedResp,
		}, nil
	}

	return map[string]any{
		"status":    "accepted",
		"protocol":  "lightspeed_open",
		"messageId": messageID,
		"threadId":  input.ThreadID,
		"otid":      otid,
		"raw":       parsedResp,
	}, nil
}
