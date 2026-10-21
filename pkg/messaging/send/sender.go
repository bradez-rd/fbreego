package send

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

const SendURL = "https://www.facebook.com/messaging/send/"

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

	otid := utils.GenThreadingID()
	now := time.Now()
	nowMillis := fmt.Sprintf("%d", now.UnixMilli())

	form := url.Values{
		"client":                      {"mercury"},
		"action_type":                 {"ma-type:user-generated-message"},
		"author":                      {"fbid:" + fb.FacebookID},
		"timestamp":                   {nowMillis},
		"timestamp_absolute":          {"Today"},
		"timestamp_relative":          {now.Format("15:04")},
		"timestamp_time_passed":       {"0"},
		"is_unread":                   {"false"},
		"is_cleared":                  {"false"},
		"is_forward":                  {"false"},
		"is_filtered_content":         {"false"},
		"is_filtered_content_bh":      {"false"},
		"is_filtered_content_account": {"false"},
		"is_filtered_content_quan":    {"false"},
		"is_spoof_warning":            {"false"},
		"source":                      {"source:chat:prompt"},
		"source_tags[0]":              {"source:chat"},
		"body":                        {input.Text},
		"ui_push_phase":               {"V3"},
		"status":                      {"0"},
		"offline_threading_id":        {otid},
		"message_id":                  {otid},
		"ephemeral_ttl_mode":          {"0"},
		"manual_retry_cnt":            {"0"},
		"has_attachment":              {"false"},
		"fb_dtsg":                     {fb.FbDtsg},
		"jazoest":                     {fb.Jazoest},
		"__user":                      {fb.FacebookID},
		"__a":                         {"1"},
		"__req":                       {utils.NextRequestBase36()},
		"__rev":                       {fb.ClientRevision},
	}

	if input.ThreadType == "group" {
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

	req, err := http.NewRequestWithContext(ctx, "POST", SendURL, strings.NewReader(form.Encode()))
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

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error al leer respuesta de envío: %w", err)
	}

	parsed, err := utils.ParseFacebookJSON(bodyBytes, true)
	if err != nil {
		return nil, err
	}

	return parsed, nil
}
