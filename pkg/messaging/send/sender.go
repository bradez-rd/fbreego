package send

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/contracts"
	"github.com/bradez-rd/fbreego/pkg/core/httputil"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
	"github.com/bradez-rd/fbreego/pkg/messaging/e2ee"
)

// SendURL define la URL canónica para envío de mensajes desde contratos.
var SendURL = contracts.SendURL()

// SendText envía un mensaje de texto plano a un usuario o conversación grupal.
func SendText(ctx context.Context, client *http.Client, fb *types.DataFB, input types.TextMessageInput) (map[string]any, error) {
	return sendMessage(ctx, client, fb, input, nil)
}

// SendWithAttachments sends a message and binds previously uploaded attachment IDs.
func SendWithAttachments(ctx context.Context, client *http.Client, fb *types.DataFB, input types.TextMessageInput, uploaded []*types.AttachmentResult) (map[string]any, error) {
	if len(uploaded) == 0 {
		return nil, fmt.Errorf("se requiere al menos un adjunto cargado")
	}
	return sendMessage(ctx, client, fb, input, uploaded)
}

func sendMessage(ctx context.Context, client *http.Client, fb *types.DataFB, input types.TextMessageInput, uploaded []*types.AttachmentResult) (map[string]any, error) {
	if fb == nil {
		return nil, fmt.Errorf("datos de sesión no configurados")
	}
	if strings.TrimSpace(input.ThreadID) == "" {
		return nil, fmt.Errorf("threadId es obligatorio")
	}
	if strings.TrimSpace(input.Text) == "" && len(uploaded) == 0 {
		return nil, fmt.Errorf("el mensaje requiere texto o adjuntos")
	}
	if client == nil {
		client = http.DefaultClient
	}

	// 0. Despacho E2EE si el hilo es explícitamente E2EE, termina en @msgr o contiene e2ee
	if input.ThreadType == "e2ee" || strings.HasSuffix(input.ThreadID, "@msgr") || strings.Contains(input.ThreadID, "e2ee") {
		km, err := e2ee.DefaultKeyManager()
		if err == nil {
			backend := e2ee.NewMessengerE2EEBackendWithKeyManager(km, client)
			return backend.Call(ctx, "sendE2EETextMessage", map[string]any{
				"chatJid": input.ThreadID,
				"text":    input.Text,
				"session": fb,
			})
		}
	}

	// 1. Despacho moderno vía LightSpeed para hilos abiertos explícitos (Marketplace, Open Messenger, o LightSpeed)
	if input.ThreadType == "lightspeed" || input.ThreadType == "open" || input.ThreadType == "marketplace" {
		var attachmentIDs []int64
		for _, a := range uploaded {
			if a != nil {
				if id, err := strconv.ParseInt(strings.TrimSpace(a.AttachmentID), 10, 64); err == nil {
					attachmentIDs = append(attachmentIDs, id)
				}
			}
		}

		res, err := SendLightSpeedOpen(ctx, client, fb, input, attachmentIDs)
		if err != nil && errors.Is(err, ErrCutoverToE2EE) {
			km, kErr := e2ee.DefaultKeyManager()
			if kErr == nil {
				backend := e2ee.NewMessengerE2EEBackendWithKeyManager(km, client)
				return backend.Call(ctx, "sendE2EETextMessage", map[string]any{
					"chatJid": input.ThreadID,
					"text":    input.Text,
					"session": fb,
				})
			}
		}
		return res, err
	}

	// 2. Despacho tradicional / fallback Mercury para compatibilidad existente
	otid := utils.GenThreadingID()
	now := time.Now()
	nowMillis := fmt.Sprintf("%d", now.UnixMilli())
	threadingID := fmt.Sprintf("<%s:%d-%x@mail.projektitan.com>", nowMillis, rand.Uint32(), rand.Int31())

	form := url.Values{
		"client":                          {"mercury"},
		"action_type":                     {"ma-type:user-generated-message"},
		"author":                          {"fbid:" + fb.FacebookID},
		"timestamp":                       {nowMillis},
		"timestamp_absolute":              {"Today"},
		"timestamp_relative":              {now.Format("15:04")},
		"timestamp_time_passed":           {"0"},
		"is_unread":                       {"false"},
		"is_cleared":                      {"false"},
		"is_forward":                      {"false"},
		"is_filtered_content":             {"false"},
		"is_filtered_content_bh":          {"false"},
		"is_filtered_content_account":     {"false"},
		"is_filtered_content_quasar":      {"false"},
		"is_filtered_content_invalid_app": {"false"},
		"is_spoof_warning":                {"false"},
		"source":                          {"source:chat:web"},
		"source_tags[0]":                  {"source:chat"},
		"client_thread_id":                {"root:" + otid},
		"body":                            {input.Text},
		"ui_push_phase":                   {"V3"},
		"status":                          {"0"},
		"offline_threading_id":            {otid},
		"message_id":                      {otid},
		"threading_id":                    {threadingID},
		"ephemeral_ttl_mode":              {"0"},
		"manual_retry_cnt":                {"0"},
		"has_attachment":                  {"false"},
		"fb_dtsg":                         {fb.FbDtsg},
		"jazoest":                         {fb.Jazoest},
		"__user":                          {fb.FacebookID},
		"av":                              {fb.FacebookID},
		"__a":                             {"1"},
		"__req":                           {utils.NextRequestBase36()},
		"__rev":                           {fb.ClientRevision},
	}

	if types.NormalizeThreadType(input.ThreadType) == types.ThreadTypeGroup {
		form.Set("thread_fbid", input.ThreadID)
	} else {
		form.Set("other_user_fbid", input.ThreadID)
		form.Set("specific_to_list[0]", "fbid:"+input.ThreadID)
		form.Set("specific_to_list[1]", "fbid:"+fb.FacebookID)
	}

	if input.ReplyToMessageID != "" {
		form.Set("replied_to_message_id", input.ReplyToMessageID)
	}

	if len(uploaded) > 0 {
		form.Set("has_attachment", "true")
		indexes := map[string]int{}
		for _, attachment := range uploaded {
			if attachment == nil || strings.TrimSpace(attachment.AttachmentID) == "" {
				return nil, fmt.Errorf("adjunto cargado sin identificador")
			}
			field := "file_ids"
			switch attachment.AttachmentType {
			case "image":
				field = "image_ids"
			case "video":
				field = "video_ids"
			case "audio":
				field = "audio_ids"
			case "file":
			default:
				return nil, fmt.Errorf("tipo de adjunto no soportado: %s", attachment.AttachmentType)
			}
			index := indexes[field]
			form.Set(fmt.Sprintf("%s[%d]", field, index), attachment.AttachmentID)
			indexes[field] = index + 1
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", contracts.SendURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("error al construir petición de envío: %w", err)
	}

	headers := utils.DefaultHeaders("www.facebook.com")
	for k, v := range headers {
		req.Header[k] = v
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fb.CookieFacebook)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fallo de red al enviar mensaje: %w", err)
	}
	defer resp.Body.Close()

	if err := httputil.CheckHTTPResponse(resp); err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error al leer respuesta de envío: %w", err)
	}

	parsed, err := utils.ParseFacebookJSON(bodyBytes, true)
	if err != nil {
		return nil, fmt.Errorf("error al decodificar respuesta JSON de facebook (FinalURL: %s, HTTP %d, body: %q): %w", resp.Request.URL.String(), resp.StatusCode, string(bodyBytes), err)
	}

	if errVal, ok := parsed["error"]; ok {
		errDesc, _ := parsed["errorDescription"].(string)
		if errDesc == "" {
			errDesc, _ = parsed["errorSummary"].(string)
		}
		if errDesc == "" {
			errDesc = fmt.Sprintf("código de error de Facebook: %v", errVal)
		}
		return nil, fmt.Errorf("facebook rechazó el envío (%v): %s", errVal, errDesc)
	}
	if payload, ok := parsed["payload"].(map[string]any); ok {
		if errVal, ok := payload["error-code"]; ok {
			errDesc, _ := payload["error-description"].(string)
			return nil, fmt.Errorf("facebook rechazó el envío (%v): %s", errVal, errDesc)
		}
	}

	if errDesc, ok := parsed["errorDescription"].(string); ok && errDesc != "" {
		errSum, _ := parsed["errorSummary"].(string)
		return nil, fmt.Errorf("facebook rechazó el envío: %s (%s)", errDesc, errSum)
	}
	if jsmods, ok := parsed["jsmods"].(map[string]any); ok {
		if require, ok := jsmods["require"].([]any); ok {
			for _, reqItem := range require {
				if rArr, ok := reqItem.([]any); ok && len(rArr) >= 2 && rArr[0] == "ServerRedirect" {
					return nil, fmt.Errorf("la cuenta de Facebook requiere verificación de seguridad (checkpoint)")
				}
			}
		}
	}

	return parsed, nil
}
