package lightspeed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

var scriptJSONRegex = regexp.MustCompile(`<script[^>]*>\s*({"require":\[\["ScheduledServerJS".*?\]\]})\s*</script>`)

// ThreadSummary representa los metadatos resumidos de un hilo de conversación de Messenger.
type ThreadSummary struct {
	ThreadID       string `json:"threadId"`
	Name           string `json:"name"`
	IsGroup        bool   `json:"isGroup"`
	Snippet        string `json:"snippet"`
	LastActivityTs int64  `json:"lastActivityTs"`
	UnreadCount    int    `json:"unreadCount"`
}

// ParseLightSpeedDataFromHTML busca y extrae los bloques JSON de LightSpeed incrustados en la página web de Messenger.
func ParseLightSpeedDataFromHTML(html string) []map[string]any {
	var results []map[string]any

	// Búsqueda de scripts con ScheduledServerJS o payloads de RelayPrefetchedStreamCache
	matches := scriptJSONRegex.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) > 1 {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(m[1]), &parsed); err == nil {
				results = append(results, parsed)
			}
		}
	}

	return results
}

// FetchLightSpeedData descarga la página inicial de Messenger (`/messages/t/`) y procesa los datos iniciales de LightSpeed.
func FetchLightSpeedData(ctx context.Context, client *http.Client, cookies string) ([]ThreadSummary, error) {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.facebook.com/messages/t/", nil)
	if err != nil {
		return nil, err
	}

	headers := utils.DefaultHeaders("www.facebook.com")
	for k, v := range headers {
		req.Header[k] = v
	}
	req.Header.Set("Cookie", cookies)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error al obtener /messages/t/: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	_ = ParseLightSpeedDataFromHTML(string(bodyBytes))

	// Por ahora retornamos una lista vacía inicial si no se encuentran threads en el HTML estático
	var summaries []ThreadSummary
	return summaries, nil
}
