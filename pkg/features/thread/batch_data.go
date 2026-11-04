package thread

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
	if client == nil {
		client = http.DefaultClient
	}

	docID := contracts.DocID("ThreadListQuery")
	if docID == "" {
		docID = contracts.DefaultDocIDThreadList
	}
	queriesJSON := fmt.Sprintf(`{"o0":{"doc_id":"%s","query_params":{"limit":50,"before":null,"tags":["INBOX"],"includeDeliveryReceipts":false,"includeSeqID":true}}}`, docID)

	form := utils.BuildFormAll(fb, "", "", false)
	form.Set("queries", queriesJSON)

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
		if tk != nil {
			if v, ok := tk["thread_fbid"].(string); ok && v != "" {
				tid = v
			} else if v, ok := tk["other_user_id"].(string); ok && v != "" {
				tid = v
			} else if v, ok := tk["other_user_id"].(float64); ok && v > 0 {
				tid = fmt.Sprintf("%.0f", v)
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
		if strings.TrimSpace(name) == "" && tid != "" {
			name = "Chat " + tid
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
