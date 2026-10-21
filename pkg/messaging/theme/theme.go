package theme

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bradez-rd/fbreego/pkg/core/httputil"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

const (
	GraphQLURL   = "https://www.facebook.com/api/graphql/"
	ThemeQueryID = "1761643933953838"
)

// ThemeMetadata describe un tema gráfico disponible en Facebook Messenger.
type ThemeMetadata struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	AccessibilityLabel string `json:"accessibilityLabel"`
	Key                string `json:"key"`
}

// ListThemes consulta a la API de Facebook la lista completa de temas gráficos disponibles.
func ListThemes(ctx context.Context, client *http.Client, fb *types.DataFB) ([]ThemeMetadata, error) {
	if client == nil {
		client = http.DefaultClient
	}

	variables := map[string]any{
		"thread_id": fb.FacebookID,
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "MessagingThemeQuery", ThemeQueryID, true)
	form.Set("variables", string(varBytes))

	req, err := http.NewRequestWithContext(ctx, "POST", GraphQLURL, strings.NewReader(form.Encode()))
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
		return nil, fmt.Errorf("error al consultar temas de messenger: %w", err)
	}
	defer resp.Body.Close()

	if err := httputil.CheckHTTPResponse(resp); err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	parsed, err := utils.ParseFacebookJSON(bodyBytes, true)
	if err != nil {
		return nil, err
	}

	var results []ThemeMetadata
	dataMap, _ := parsed["data"].(map[string]any)
	if dataMap == nil {
		return results, nil
	}
	themesRoot, _ := dataMap["messenger_thread_themes"].(map[string]any)
	if themesRoot == nil {
		return results, nil
	}
	edges, _ := themesRoot["theme_edges"].([]any)
	for _, edge := range edges {
		eMap, ok := edge.(map[string]any)
		if !ok {
			continue
		}
		node, ok := eMap["node"].(map[string]any)
		if !ok {
			continue
		}
		id := fmt.Sprintf("%v", node["theme_id"])
		name, _ := node["theme_name"].(string)
		label, _ := node["accessibility_label"].(string)
		key, _ := node["theme_key"].(string)
		if id != "" {
			results = append(results, ThemeMetadata{
				ID:                 id,
				Name:               name,
				AccessibilityLabel: label,
				Key:                key,
			})
		}
	}

	return results, nil
}
