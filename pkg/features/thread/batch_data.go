package thread

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

const BatchQueryURL = "https://www.facebook.com/api/graphqlbatch/"

// FetchAllThreads obtiene mediante una consulta GraphQLBatch la lista estructurada de conversaciones de la cuenta.
func FetchAllThreads(ctx context.Context, client *http.Client, fb *types.DataFB) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	form := url.Values{
		"fb_dtsg": {fb.FbDtsg},
		"jazoest": {fb.Jazoest},
		"__user":  {fb.FacebookID},
		"__a":     {"1"},
		"__req":   {utils.NextRequestBase36()},
		"__rev":   {fb.ClientRevision},
		"queries": {`{"o0":{"doc_id":"3336396659757871","query_params":{"limit":50,"before":null,"tags":["INBOX"],"includeDeliveryReceipts":false,"includeSeqID":true}}}`},
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
		return nil, fmt.Errorf("error al consultar conversaciones batch: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return utils.ParseGraphQLBatchResponse(bodyBytes)
}

// ExtractThreadItems normaliza la respuesta cruda de GraphQL batch en una lista de hilos de fbreego.
func ExtractThreadItems(threadsData map[string]any) []map[string]any {
	var items []map[string]any
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
