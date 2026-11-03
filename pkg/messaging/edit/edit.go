package edit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

const BatchQueryURL = "https://www.facebook.com/api/graphqlbatch/"

// BuildEditContext construye el payload de tarea LightSpeed para editar un mensaje existente.
func BuildEditContext(messageID, newText string) (string, error) {
	taskPayload := map[string]any{
		"message_id": messageID,
		"text":       newText,
	}
	taskPayloadBytes, err := json.Marshal(taskPayload)
	if err != nil {
		return "", err
	}

	lsPayload := map[string]any{
		"tasks": []map[string]any{
			{
				"label":      "22",
				"payload":    string(taskPayloadBytes),
				"queue_name": "edit_message",
				"task_id":    1,
			},
		},
		"epoch_id":   utils.GenerateSessionID(),
		"version_id": "7191105584282302",
	}

	lsPayloadBytes, err := json.Marshal(lsPayload)
	if err != nil {
		return "", err
	}

	wrapper := map[string]any{
		"label":   "21",
		"payload": string(lsPayloadBytes),
	}

	wrapperBytes, err := json.Marshal(wrapper)
	if err != nil {
		return "", err
	}

	return string(wrapperBytes), nil
}

// EditMessage envía a Facebook la tarea de edición de un mensaje.
func EditMessage(ctx context.Context, client *http.Client, fb *types.DataFB, messageID, newText string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	editContext, err := BuildEditContext(messageID, newText)
	if err != nil {
		return nil, fmt.Errorf("error construyendo contexto de edición: %w", err)
	}

	form := url.Values{
		"fb_dtsg":  {fb.FbDtsg},
		"jazoest":  {fb.Jazoest},
		"__user":   {fb.FacebookID},
		"__a":      {"1"},
		"__req":    {utils.NextRequestBase36()},
		"__rev":    {fb.ClientRevision},
		"queries":  {fmt.Sprintf(`{"o0":{"doc_id":"7191105584282302","query_params":{"context":%s}}}`, editContext)},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", BatchQueryURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	headers := utils.DefaultHeaders("www.facebook.com")
	for k, v := range headers {
		req.Header[k] = v
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fb.CookieFacebook)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error al enviar edición de mensaje: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	res, err := utils.ParseFacebookJSON(bodyBytes, true)
	if err != nil {
		if resp.StatusCode == http.StatusOK {
			return map[string]any{
				"messageId": messageID,
				"text":      newText,
				"status":    "edited",
			}, nil
		}
		return nil, err
	}
	res["messageId"] = messageID
	res["text"] = newText
	res["status"] = "edited"
	return res, nil
}
