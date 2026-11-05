package thread

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
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

	attachments := make([]map[string]any, 0)
	if sticker, ok := node["sticker"].(map[string]any); ok {
		if normalized := normalizeGraphQLSticker(sticker); normalized != nil {
			attachments = append(attachments, normalized)
		}
	}
	if len(attachments) == 0 {
		if blobAtts, ok := node["blob_attachments"].([]any); ok {
			for _, a := range blobAtts {
				if attMap, ok := a.(map[string]any); ok {
					if normalized := normalizeGraphQLBlobAttachment(attMap); normalized != nil {
						attachments = append(attachments, normalized)
					}
				}
			}
		}
	}
	if len(attachments) == 0 {
		if extensible, ok := node["extensible_attachment"].(map[string]any); ok {
			if normalized := normalizeGraphQLExtensibleAttachment(extensible); normalized != nil {
				attachments = append(attachments, normalized)
			}
		}
	}
	if text == "" && len(attachments) > 0 && stringField(attachments[0], "type") == "link" {
		text = firstString(
			stringField(attachments[0], "title"),
			stringField(attachments[0], "description"),
			stringField(attachments[0], "url"),
		)
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

func normalizeGraphQLBlobAttachment(source map[string]any) map[string]any {
	typename := stringField(source, "__typename")
	filename := stringField(source, "filename")
	extension := strings.TrimPrefix(stringField(source, "original_extension"), ".")
	result := map[string]any{
		"id": stringField(source, "legacy_attachment_id", "message_file_fbid", "url_shimhash"),
	}

	switch typename {
	case "MessageImage":
		result["type"] = "image"
		result["url"] = firstString(nestedString(source, "large_preview", "uri"), nestedString(source, "preview", "uri"), nestedString(source, "thumbnail", "uri"))
		result["thumbnailUrl"] = firstString(nestedString(source, "thumbnail", "uri"), nestedString(source, "preview", "uri"))
		result["mimeType"] = imageMimeType(extension)
		filename = ensureExtension(filename, extension)
	case "MessageAnimatedImage":
		result["type"] = "image"
		result["url"] = nestedString(source, "animated_image", "uri")
		result["thumbnailUrl"] = nestedString(source, "preview_image", "uri")
		result["mimeType"] = "image/gif"
		filename = ensureExtension(filename, "gif")
	case "MessageVideo":
		result["type"] = "video"
		result["url"] = stringField(source, "playable_url")
		result["thumbnailUrl"] = nestedString(source, "large_image", "uri")
		result["mimeType"] = "video/mp4"
		result["durationMs"] = source["playable_duration_in_ms"]
	case "MessageAudio":
		result["type"] = "audio"
		result["url"] = stringField(source, "playable_url")
		result["mimeType"] = "audio/mpeg"
		result["durationMs"] = source["playable_duration_in_ms"]
	case "MessageFile":
		result["type"] = "document"
		result["url"] = stringField(source, "url")
		result["mimeType"] = stringField(source, "content_type")
	default:
		return nil
	}

	if filename != "" {
		result["fileName"] = filename
	}
	if stringField(result, "url") == "" {
		return nil
	}
	return compactAttachment(result)
}

func normalizeGraphQLSticker(source map[string]any) map[string]any {
	url := firstString(stringField(source, "url"), stringField(source, "sprite_image_2x"), stringField(source, "sprite_image"))
	if url == "" {
		return nil
	}
	return compactAttachment(map[string]any{
		"type":         "sticker",
		"id":           stringField(source, "id"),
		"url":          url,
		"thumbnailUrl": firstString(stringField(source, "sprite_image"), url),
		"mimeType":     "image/webp",
		"fileName":     "sticker.webp",
	})
}

func normalizeGraphQLExtensibleAttachment(source map[string]any) map[string]any {
	story, ok := source["story_attachment"].(map[string]any)
	if !ok {
		return nil
	}
	media, _ := story["media"].(map[string]any)
	imageURL := firstString(nestedString(media, "animated_image", "uri"), nestedString(media, "image", "uri"))
	url := stringField(story, "url")
	if url == "" && imageURL == "" {
		return nil
	}
	return compactAttachment(map[string]any{
		"type":         "link",
		"id":           stringField(source, "legacy_attachment_id"),
		"url":          url,
		"title":        nestedString(story, "title_with_entities", "text"),
		"description":  nestedString(story, "description", "text"),
		"thumbnailUrl": imageURL,
	})
}

func stringField(source map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := source[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func nestedString(source map[string]any, objectKey, valueKey string) string {
	if source == nil {
		return ""
	}
	nested, ok := source[objectKey].(map[string]any)
	if !ok {
		return ""
	}
	return stringField(nested, valueKey)
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func ensureExtension(filename, extension string) string {
	extension = strings.TrimPrefix(strings.TrimSpace(extension), ".")
	if filename == "" || extension == "" || filepath.Ext(filename) != "" {
		return filename
	}
	return filename + "." + extension
}

func imageMimeType(extension string) string {
	switch strings.ToLower(strings.TrimPrefix(extension, ".")) {
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	default:
		return "image/jpeg"
	}
}

func compactAttachment(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				result[key] = typed
			}
		case nil:
			continue
		default:
			result[key] = value
		}
	}
	return result
}
