package thread

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/bradez-rd/fbreego/pkg/core/contracts"
	"github.com/bradez-rd/fbreego/pkg/core/httputil"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

// ThreadMessage representa la estructura canónica de un mensaje remoto de Facebook.
type ThreadMessage struct {
	MessageID   string           `json:"messageId"`
	ThreadID    string           `json:"threadId"`
	SenderID    string           `json:"senderId"`
	Text        string           `json:"text"`
	Timestamp   int64            `json:"timestamp"`
	Attachments []map[string]any `json:"attachments"`
	IsFromMe    bool             `json:"isFromMe"`
}

// FetchThreadMessages consulta el historial de mensajes de un hilo mediante MessengerGraphQLThreadFetcher.
func FetchThreadMessages(ctx context.Context, client *http.Client, fb *types.DataFB, threadID string, limit int, before string) ([]ThreadMessage, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if limit <= 0 {
		limit = 20
	}

	queryParams := map[string]any{
		"id":            threadID,
		"message_limit": limit,
	}
	if before != "" {
		if ts, err := strconv.ParseInt(before, 10, 64); err == nil {
			queryParams["load_messages_before_time"] = ts
		} else {
			queryParams["before"] = before
		}
	}

	queryParamsJSON, err := json.Marshal(queryParams)
	if err != nil {
		return nil, fmt.Errorf("error serializando queryParams: %w", err)
	}

	queries := fmt.Sprintf(`{"o0":{"doc_id":"3449967031715030","query_params":%s}}`, string(queryParamsJSON))

	form := url.Values{
		"fb_dtsg": {fb.FbDtsg},
		"jazoest": {fb.Jazoest},
		"__user":  {fb.FacebookID},
		"__a":     {"1"},
		"__req":   {utils.NextRequestBase36()},
		"__rev":   {fb.ClientRevision},
		"queries": {queries},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", contracts.BatchQueryURL(), strings.NewReader(form.Encode()))
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
		return nil, fmt.Errorf("error de red al consultar mensajes de hilo: %w", err)
	}
	defer resp.Body.Close()

	if err := httputil.CheckHTTPResponse(resp); err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	parsed, err := utils.ParseGraphQLBatchResponse(bodyBytes)
	if err != nil {
		parsed, err = utils.ParseFacebookJSON(bodyBytes, true)
		if err != nil {
			return nil, err
		}
	}

	var messages []ThreadMessage
	// Intentar extraer nodos del o0 o payload batch
	if o0, ok := parsed["o0"].(map[string]any); ok {
		if data, ok := o0["data"].(map[string]any); ok {
			if threadObj, ok := data["message_thread"].(map[string]any); ok {
				if msgsObj, ok := threadObj["messages"].(map[string]any); ok {
					if nodes, ok := msgsObj["nodes"].([]any); ok {
						for _, n := range nodes {
							if node, ok := n.(map[string]any); ok {
								msg := parseGraphQLMessageNode(node, threadID, fb.FacebookID)
								if msg.MessageID != "" {
									messages = append(messages, msg)
								}
							}
						}
					}
				}
			}
		}
	}

	return messages, nil
}

func parseGraphQLMessageNode(node map[string]any, defaultThreadID, ownFacebookID string) ThreadMessage {
	msgID, _ := node["id"].(string)
	if msgID == "" {
		msgID, _ = node["message_id"].(string)
	}

	var senderID string
	if sender, ok := node["message_sender"].(map[string]any); ok {
		if id, ok := sender["id"].(string); ok {
			senderID = id
		}
	}

	var text string
	if msgText, ok := node["message"].(map[string]any); ok {
		text, _ = msgText["text"].(string)
	} else if snippet, ok := node["snippet"].(string); ok {
		text = snippet
	}

	var timestamp int64
	if tsStr, ok := node["timestamp_precise"].(string); ok {
		timestamp, _ = strconv.ParseInt(tsStr, 10, 64)
	} else if tsNum, ok := node["timestamp_precise"].(float64); ok {
		timestamp = int64(tsNum)
	}

	var attachments []map[string]any
	if blobAtts, ok := node["blob_attachments"].([]any); ok {
		for _, a := range blobAtts {
			if attMap, ok := a.(map[string]any); ok {
				attachments = append(attachments, attMap)
			}
		}
	}

	isFromMe := senderID != "" && senderID == ownFacebookID

	return ThreadMessage{
		MessageID:   msgID,
		ThreadID:    defaultThreadID,
		SenderID:    senderID,
		Text:        text,
		Timestamp:   timestamp,
		Attachments: attachments,
		IsFromMe:    isFromMe,
	}
}
