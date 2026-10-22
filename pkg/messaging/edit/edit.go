package edit

import (
	"encoding/json"

	"github.com/bradez-rd/fbreego/pkg/core/contracts"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

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

	versionID := contracts.DocID("EditMessageVersion")
	if versionID == "" {
		versionID = contracts.DefaultVersionIDEditMessage
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
		"version_id": versionID,
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
