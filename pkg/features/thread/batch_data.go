package thread

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bradez-rd/fbreego/pkg/core/contracts"
	"github.com/bradez-rd/fbreego/pkg/core/httputil"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

// BatchQueryURL se mantiene como constante por compatibilidad con consumidores anteriores.
// Las peticiones internas usan el registro de contratos para admitir overrides en runtime.
const BatchQueryURL = contracts.DefaultBatchQueryURL

// FetchAllThreads obtiene mediante una consulta GraphQLBatch la lista estructurada de conversaciones de la cuenta.
func FetchAllThreads(ctx context.Context, client *http.Client, fb *types.DataFB) (map[string]any, error) {
	return FetchThreadPage(ctx, client, fb, 50, "")
}

// FetchThreadPage consulta una página GraphQL anterior a la marca temporal indicada.
func FetchThreadPage(ctx context.Context, client *http.Client, fb *types.DataFB, limit int, before string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	docID := contracts.DocID("ThreadListQuery")
	if docID == "" {
		docID = contracts.DefaultDocIDThreadList
	}
	var cursor any
	if before != "" {
		parsed, err := strconv.ParseInt(before, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("cursor de hilos inválido")
		}
		cursor = parsed
	}
	query, err := json.Marshal(map[string]any{
		"o0": map[string]any{
			"doc_id": docID,
			"query_params": map[string]any{
				"limit": limit, "before": cursor, "tags": []string{"INBOX"},
				"includeDeliveryReceipts": false, "includeSeqID": true,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "", "", false)
	form.Set("queries", string(query))

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
		return nil, fmt.Errorf("error al consultar conversaciones batch: %w", err)
	}
	defer resp.Body.Close()

	if err := httputil.CheckHTTPResponse(resp); err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return utils.ParseGraphQLBatchResponse(bodyBytes)
}

// ThreadPageInfo devuelve nil para hasMore si Facebook no expone page_info.
// El cursor de avance procede de updated_time_precise, usado por la consulta GraphQL.
func ThreadPageInfo(threadsData map[string]any, limit int) (hasMore *bool, nextCursor string) {
	mt := messageThreads(threadsData)
	nodes, _ := mt["nodes"].([]any)
	if len(nodes) > 0 {
		var oldest int64
		for _, raw := range nodes {
			node, _ := raw.(map[string]any)
			value, _ := node["updated_time_precise"].(string)
			timestamp, err := strconv.ParseInt(value, 10, 64)
			if err == nil && timestamp > 0 && (oldest == 0 || timestamp < oldest) {
				oldest = timestamp
			}
		}
		if oldest > 0 {
			nextCursor = strconv.FormatInt(oldest, 10)
		}
	}
	page, _ := mt["page_info"].(map[string]any)
	if page == nil {
		if len(nodes) >= limit && limit > 0 {
			return nil, nextCursor
		}
		return nil, ""
	}
	more, ok := page["has_next_page"].(bool)
	if !ok {
		if len(nodes) >= limit && limit > 0 {
			return nil, nextCursor
		}
		return nil, ""
	}
	if more {
		if nextCursor == "" {
			return nil, ""
		}
		return &more, nextCursor
	}
	return &more, ""
}

func messageThreads(threadsData map[string]any) map[string]any {
	o0, _ := threadsData["o0"].(map[string]any)
	if o0 == nil {
		o0 = threadsData
	}
	data, _ := o0["data"].(map[string]any)
	viewer, _ := data["viewer"].(map[string]any)
	mt, _ := viewer["message_threads"].(map[string]any)
	return mt
}

// ExtractThreadItems normaliza la respuesta cruda de GraphQL batch en una lista de hilos de fbreego.
func ExtractThreadItems(threadsData map[string]any) []map[string]any {
	items := make([]map[string]any, 0)
	if threadsData == nil {
		return items
	}

	o0, _ := threadsData["o0"].(map[string]any)
	if o0 == nil {
		o0 = threadsData
	}
	data, _ := o0["data"].(map[string]any)
	if data == nil {
		return items
	}
	viewer, _ := data["viewer"].(map[string]any)
	if viewer == nil {
		return items
	}
	mt, _ := viewer["message_threads"].(map[string]any)
	if mt == nil {
		return items
	}
	nodes, _ := mt["nodes"].([]any)
	for _, n := range nodes {
		node, ok := n.(map[string]any)
		if !ok {
			continue
		}
		tk, _ := node["thread_key"].(map[string]any)
		var tid string
		var recipientID string
		if tk != nil {
			if v, ok := tk["thread_fbid"].(string); ok && v != "" {
				tid = v
			}
			if v, ok := tk["other_user_id"].(string); ok && v != "" {
				recipientID = v
			} else if v, ok := tk["other_user_id"].(float64); ok && v > 0 {
				recipientID = fmt.Sprintf("%.0f", v)
			}
			if tid == "" {
				tid = recipientID
			}
		}
		if tid == "" {
			if idStr, ok := node["id"].(string); ok {
				tid = idStr
			}
		}

		name, _ := node["name"].(string)

		// En hilos 1:1 de Facebook GraphQL, el campo `name` suele venir vacío/nulo.
		// Extraemos el nombre del participante desde `all_participants`.
		if strings.TrimSpace(name) == "" {
			if ap, ok := node["all_participants"].(map[string]any); ok {
				if edges, ok := ap["edges"].([]any); ok {
					for _, edgeItem := range edges {
						if edge, ok := edgeItem.(map[string]any); ok {
							if nodeActor, ok := edge["node"].(map[string]any); ok {
								if actor, ok := nodeActor["messaging_actor"].(map[string]any); ok {
									actorID := fmt.Sprintf("%v", actor["id"])
									actorName, _ := actor["name"].(string)
									if strings.TrimSpace(actorName) != "" {
										if name == "" {
											name = actorName
										}
										if tid != "" && actorID == tid {
											name = actorName
											break
										}
									}
								}
							}
						}
					}
				}
			}
		}
		threadTypeStr, _ := node["thread_type"].(string)
		threadType := "user"
		isMarketplace := false
		if strings.EqualFold(threadTypeStr, "GROUP") {
			threadType = "group"
		} else if strings.Contains(name, "·") || strings.Contains(strings.ToLower(name), "marketplace") {
			threadType = "marketplace"
			isMarketplace = true
		}

		var unreadCount int
		if uc, ok := node["unread_count"].(float64); ok {
			unreadCount = int(uc)
		} else if uc, ok := node["unread_count"].(int); ok {
			unreadCount = uc
		}

		updatedAt, _ := node["updated_time_precise"].(string)

		items = append(items, map[string]any{
			"threadId":    tid,
			"name":        name,
			"type":        threadType,
			"recipientId": recipientID,
			"chatJid":     recipientID,
			"unreadCount": unreadCount,
			"updatedAt":   updatedAt,
			"marketplace": isMarketplace,
		})
	}
	return items
}

// ExtractSyncSequenceID extrae el sync_sequence_id devuelto por la consulta GraphQLBatch para sincronización inicial MQTT.
func ExtractSyncSequenceID(threadsData map[string]any) int64 {
	if threadsData == nil {
		return 0
	}
	o0, _ := threadsData["o0"].(map[string]any)
	if o0 == nil {
		o0 = threadsData
	}
	data, _ := o0["data"].(map[string]any)
	if data == nil {
		return 0
	}
	viewer, _ := data["viewer"].(map[string]any)
	if viewer == nil {
		return 0
	}
	mt, _ := viewer["message_threads"].(map[string]any)
	if mt == nil {
		return 0
	}

	switch val := mt["sync_sequence_id"].(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	case string:
		var parsed int64
		if _, err := fmt.Sscanf(strings.TrimSpace(val), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return 0
}
