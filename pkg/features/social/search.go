package social

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

// SearchUsers busca personas y contactos en Facebook utilizando palabras clave.
func SearchUsers(ctx context.Context, client *http.Client, fb *types.DataFB, query string) ([]map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	variables := map[string]any{
		"search_query": query,
		"call_site":    "COMET_GLOBAL_SEARCH",
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "SearchCometResultsPaginatedQuery", "3519284019284717", true)
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
	searchRoot, _ := dataMap["serpResponse"].(map[string]any)
	if searchRoot == nil {
		return results, nil
	}
	resultsRoot, _ := searchRoot["results"].(map[string]any)
	if resultsRoot == nil {
		return results, nil
	}
	edges, _ := resultsRoot["edges"].([]any)
	for _, edge := range edges {
		if eMap, ok := edge.(map[string]any); ok {
			results = append(results, eMap)
		}
	}
	return results, nil
}

// GetUserInfo consulta los datos públicos del perfil de un usuario específico por su ID.
func GetUserInfo(ctx context.Context, client *http.Client, fb *types.DataFB, userID string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	variables := map[string]any{
		"user_id": userID,
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "ProfileCometHeaderQuery", "3419284019284718", true)
	form.Set("variables", string(varBytes))

	res, err := executePost(ctx, client, fb, GraphQLURL, form)
	if err != nil {
		return nil, err
	}

	dataMap, _ := res["data"].(map[string]any)
	if dataMap == nil {
		return nil, fmt.Errorf("información de usuario no encontrada: %s", userID)
	}
	return dataMap, nil
}
