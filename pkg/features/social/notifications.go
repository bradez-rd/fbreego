package social

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

// GetNotifications consulta la lista de notificaciones recientes de la cuenta.
func GetNotifications(ctx context.Context, client *http.Client, fb *types.DataFB) ([]map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	variables := map[string]any{
		"count": 20,
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "NotificationsCometListQuery", "3319284019284719", true)
	form.Set("variables", string(varBytes))

	res, err := executePost(ctx, client, fb, GraphQLURL, form)
	if err != nil {
		return nil, err
	}

	results := make([]map[string]any, 0)
	dataMap, _ := res["data"].(map[string]any)
	if dataMap == nil {
		return results, nil
	}
	viewer, _ := dataMap["viewer"].(map[string]any)
	if viewer == nil {
		return results, nil
	}
	notifs, _ := viewer["notifications"].(map[string]any)
	if notifs == nil {
		return results, nil
	}
	edges, _ := notifs["edges"].([]any)
	for _, edge := range edges {
		if eMap, ok := edge.(map[string]any); ok {
			results = append(results, eMap)
		}
	}
	return results, nil
}
