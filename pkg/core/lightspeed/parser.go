package lightspeed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/contracts"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

var scriptJSONRegex = regexp.MustCompile(`<script[^>]*>\s*({"require":\[\["ScheduledServerJS".*?\]\]})\s*</script>`)
var genericScriptRegex = regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`)

// ContactSummary representa los datos de un contacto extraído de los payloads de LightSpeed.
type ContactSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

// ThreadSummary representa los metadatos resumidos de un hilo de conversación de Messenger.
type ThreadSummary struct {
	ThreadID       string `json:"threadId"`
	Name           string `json:"name"`
	IsGroup        bool   `json:"isGroup"`
	Snippet        string `json:"snippet"`
	LastActivityTs int64  `json:"lastActivityTs"`
	UnreadCount    int    `json:"unreadCount"`
	Folder         string `json:"folder"`
	Type           string `json:"type"`
	UpdatedAt      string `json:"updatedAt"`
	Avatar         string `json:"avatar,omitempty"`
}

// ParseLightSpeedDataFromHTML busca y extrae los bloques JSON de LightSpeed incrustados en la página web de Messenger.
func ParseLightSpeedDataFromHTML(html string) []map[string]any {
	var results []map[string]any

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

func cleanLSArg(a any) any {
	if arr, ok := a.([]any); ok {
		if len(arr) == 2 {
			if num, ok := arr[0].(float64); ok && int(num) == 19 {
				return cleanLSArg(arr[1])
			}
		}
		if len(arr) == 1 {
			if num, ok := arr[0].(float64); ok && int(num) == 9 {
				return nil
			}
		}
	}
	return a
}

func extractLSCalls(node any, fn func(name string, args []any)) {
	if arr, ok := node.([]any); ok {
		if len(arr) >= 2 {
			if num, ok := arr[0].(float64); ok && int(num) == 5 {
				if callName, ok := arr[1].(string); ok {
					fn(callName, arr[2:])
				}
			}
		}
		for _, sub := range arr {
			extractLSCalls(sub, fn)
		}
	}
}

func findLSRequests(node any, fn func(ls map[string]any)) {
	if m, ok := node.(map[string]any); ok {
		if ls, ok := m["lightspeed_web_request"].(map[string]any); ok {
			fn(ls)
		}
		for _, v := range m {
			findLSRequests(v, fn)
		}
	} else if arr, ok := node.([]any); ok {
		for _, item := range arr {
			findLSRequests(item, fn)
		}
	}
}

// ExtractLightSpeedThreadsAndContacts parsea las instrucciones de LightSpeed (verifyContactRowExists, updateOrInsertThread) presentes en el HTML de Facebook.
func ExtractLightSpeedThreadsAndContacts(html string) ([]ThreadSummary, map[string]ContactSummary) {
	contacts := make(map[string]ContactSummary)
	threads := make([]ThreadSummary, 0)

	matches := genericScriptRegex.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) < 2 || !strings.Contains(m[1], "lightspeed_web_request") {
			continue
		}
		var parsed any
		if err := json.Unmarshal([]byte(m[1]), &parsed); err != nil {
			continue
		}

		findLSRequests(parsed, func(ls map[string]any) {
			payloadStr, _ := ls["payload"].(string)
			if payloadStr == "" {
				return
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
				return
			}
			steps, _ := payload["step"].([]any)

			// Paso 1: Extraer contactos desde verifyContactRowExists
			for _, st := range steps {
				extractLSCalls(st, func(name string, args []any) {
					if name == "verifyContactRowExists" {
						cleaned := make([]any, len(args))
						for i, a := range args {
							cleaned[i] = cleanLSArg(a)
						}
						if len(cleaned) >= 4 {
							uid := safeLSString(cleaned[0])
							cname := safeLSString(cleaned[3])
							avatar := safeLSString(cleaned[2])
							if uid != "" && cname != "" {
								contacts[uid] = ContactSummary{
									ID:     uid,
									Name:   cname,
									Avatar: avatar,
								}
							}
						}
					}
				})
			}

			// Paso 2: Extraer hilos desde updateOrInsertThread o deleteThenInsertThread
			for _, st := range steps {
				extractLSCalls(st, func(name string, args []any) {
					if name == "updateOrInsertThread" || name == "deleteThenInsertThread" {
						cleaned := make([]any, len(args))
						for i, a := range args {
							cleaned[i] = cleanLSArg(a)
						}
						if len(cleaned) > 10 {
							tid := safeLSString(cleaned[7])
							if tid == "" {
								return
							}

							updatedAt := safeLSString(cleaned[0])
							var lastActivityTs int64
							if ts, err := strconv.ParseInt(updatedAt, 10, 64); err == nil {
								lastActivityTs = ts
							}

							folder := strings.ToLower(safeLSString(cleaned[10]))
							if folder == "" {
								folder = "inbox"
							}

							unread := 0
							if len(cleaned) > 13 {
								if n, ok := cleaned[13].(float64); ok {
									unread = int(n)
								} else if s, ok := cleaned[13].(string); ok {
									if parsedUnread, err := strconv.Atoi(s); err == nil {
										unread = parsedUnread
									}
								}
							}

							ttypeCode := safeLSString(cleaned[9])
							isGroup := ttypeCode == "15" || ttypeCode == "2"
							ttype := "user"
							if isGroup {
								ttype = "group"
							}

							avatar := ""
							if len(cleaned) > 11 {
								avatar = safeLSString(cleaned[11])
							}

							snippet := ""
							if len(cleaned) > 40 {
								snippet = safeLSString(cleaned[40])
							}

							creatorID := ""
							if len(cleaned) > 21 {
								creatorID = safeLSString(cleaned[21])
							}

							resolvedName := ""
							if c, ok := contacts[creatorID]; ok && c.Name != "" {
								resolvedName = c.Name
							} else if c, ok := contacts[tid]; ok && c.Name != "" {
								resolvedName = c.Name
							} else if snippet != "" {
								resolvedName = snippet
							} else {
								if isGroup {
									resolvedName = "Grupo " + tid
								} else {
									resolvedName = "Chat " + tid
								}
							}

							threads = append(threads, ThreadSummary{
								ThreadID:       tid,
								Name:           resolvedName,
								IsGroup:        isGroup,
								Type:           ttype,
								Snippet:        snippet,
								LastActivityTs: lastActivityTs,
								UnreadCount:    unread,
								Folder:         folder,
								UpdatedAt:      updatedAt,
								Avatar:         avatar,
							})
						}
					}
				})
			}
		})
	}

	return threads, contacts
}

func safeLSString(v any) string {
	if v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", v))
	if s == "<nil>" || s == "[9]" || s == "[]" {
		return ""
	}
	return s
}

// FetchLightSpeedData descarga la página web de Facebook y procesa los datos iniciales de LightSpeed.
func FetchLightSpeedData(ctx context.Context, client *http.Client, cookies string) ([]ThreadSummary, map[string]ContactSummary, error) {
	fetchClient := &http.Client{
		Timeout: 25 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("demasiadas redirecciones")
			}
			req.Header.Set("Cookie", cookies)
			return nil
		},
	}
	if client != nil && client.Transport != nil {
		fetchClient.Transport = client.Transport
	}

	urlsToTry := []string{
		"https://www.facebook.com/",
		contracts.MessagesBaseURL(),
	}

	var lastErr error
	for _, targetURL := range urlsToTry {
		req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
		if err != nil {
			lastErr = err
			continue
		}

		headers := utils.DefaultHeaders("www.facebook.com")
		for k, v := range headers {
			req.Header[k] = v
		}
		req.Header.Set("Cookie", cookies)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "none")
		req.Header.Set("Sec-Fetch-User", "?1")
		req.Header.Set("Upgrade-Insecure-Requests", "1")

		resp, err := fetchClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			threads, contacts := ExtractLightSpeedThreadsAndContacts(string(bodyBytes))
			if len(threads) > 0 || len(contacts) > 0 {
				return threads, contacts, nil
			}
		} else {
			lastErr = fmt.Errorf("código HTTP %d al consultar %s", resp.StatusCode, targetURL)
		}
	}

	return make([]ThreadSummary, 0), make(map[string]ContactSummary), lastErr
}
