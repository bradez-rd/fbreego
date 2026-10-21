package api

import (
	"encoding/json"
	"net/http"
)

// OpenAPISchema describes every stable HTTP capability exposed by Routes.
func OpenAPISchema() map[string]any {
	paths := map[string]any{
		"/healthz":                              pathOps("get", publicOperation("Healthcheck endpoint")),
		"/readyz":                               pathOps("get", publicOperation("Readiness endpoint")),
		"/openapi.json":                         pathOps("get", publicOperation("OpenAPI schema")),
		"/docs":                                 pathOps("get", publicOperation("Swagger UI")),
		"/ws":                                   pathOps("get", operation("Realtime WebSocket stream")),
		"/":                                     pathOps("get", operation("Administrative portal")),
		"/api/v1/app/info":                      pathOps("get", operation("Application information")),
		"/api/v1/app/status":                    pathOps("get", operation("Application status")),
		"/api/v1/diagnostics":                   pathOps("get", operation("Runtime diagnostics")),
		"/api/v1/websocket-ticket":              pathOps("post", operation("Issue a one-time WebSocket ticket")),
		"/api/v1/sessions":                      pathOps("get", operation("List managed sessions"), "post", operation("Create and validate a session")),
		"/api/v1/sessions/{sessionId}":          pathOps("delete", operation("Delete a session")),
		"/api/v1/sessions/{sessionId}/status":   pathOps("get", operation("Get session status")),
		"/api/v1/sessions/{sessionId}/validate": pathOps("post", operation("Revalidate a session")),
		"/api/v1/sessions/{sessionId}/listen/start":         pathOps("post", unavailableOperation("Start Messenger listener (not available in this build)")),
		"/api/v1/sessions/{sessionId}/listen/stop":          pathOps("post", operation("Stop session listener")),
		"/api/v1/sessions/{sessionId}/listen/status":        pathOps("get", operation("Get listener status")),
		"/api/v1/sessions/{sessionId}/threads":              pathOps("get", operation("List conversation threads")),
		"/api/v1/sessions/{sessionId}/messages/text":        pathOps("post", operation("Send a text message")),
		"/api/v1/sessions/{sessionId}/messages/media":       pathOps("post", operation("Send uploaded media attachments")),
		"/api/v1/sessions/{sessionId}/messages/reaction":    pathOps("post", operation("Add or remove a reaction")),
		"/api/v1/sessions/{sessionId}/messages/unsend":      pathOps("post", operation("Unsend a message")),
		"/api/v1/sessions/{sessionId}/users/{userId}":       pathOps("get", operation("Get user information")),
		"/api/v1/sessions/{sessionId}/search":               pathOps("get", operation("Search users")),
		"/api/v1/sessions/{sessionId}/privacy/block":        pathOps("post", operation("Block or unblock a user")),
		"/api/v1/sessions/{sessionId}/marketplace/{itemId}": pathOps("get", operation("Get Marketplace item")),
		"/api/v1/webhook":              pathOps("get", operation("Get webhook configuration"), "put", operation("Update webhook configuration"), "delete", operation("Clear webhook configuration")),
		"/api/v1/webhook/test":         pathOps("post", operation("Probe a webhook endpoint")),
		"/api/v1/media/upload":         pathOps("post", operation("Upload media to local staging")),
		"/api/v1/media":                pathOps("get", operation("List staged media")),
		"/api/v1/media/{mediaId}":      pathOps("get", operation("Download staged media"), "delete", operation("Delete staged media")),
		"/api/v1/media/{mediaId}/info": pathOps("get", operation("Get staged media metadata")),
		"/api/v1/traffic":              pathOps("get", operation("List captured HTTP traffic"), "delete", operation("Clear captured HTTP traffic")),
		"/api/v1/traffic/stats":        pathOps("get", operation("Get traffic capture statistics")),
		"/api/v1/traffic/{id}":         pathOps("get", operation("Get captured request detail")),
	}

	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "fbchat-provider",
			"description": "Go service for the supported fbreego Facebook Messenger capabilities. Experimental test routes are intentionally omitted.",
			"version":     Version,
		},
		"security": []any{map[string]any{"basicAuth": []any{}}},
		"paths":    paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"basicAuth": map[string]any{"type": "http", "scheme": "basic"},
			},
		},
	}
}

func pathOps(entries ...any) map[string]any {
	result := make(map[string]any, len(entries)/2)
	for i := 0; i+1 < len(entries); i += 2 {
		method, _ := entries[i].(string)
		result[method] = entries[i+1]
	}
	return result
}

func operation(summary string) map[string]any {
	return map[string]any{
		"summary": summary,
		"responses": map[string]any{
			"200": map[string]any{"description": "Successful response"},
			"401": map[string]any{"description": "Authentication required"},
			"503": map[string]any{"description": "Required service configuration is missing"},
		},
	}
}

func publicOperation(summary string) map[string]any {
	op := operation(summary)
	op["security"] = []any{}
	return op
}

func unavailableOperation(summary string) map[string]any {
	op := operation(summary)
	op["responses"].(map[string]any)["501"] = map[string]any{"description": "Capability is not implemented"}
	return op
}

func (s *Server) ServeOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(OpenAPISchema())
}

func (s *Server) ServeSwaggerUI(w http.ResponseWriter, r *http.Request) {
	const html = `<!DOCTYPE html>
<html>
<head>
    <title>fbchat-provider - Swagger UI</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5/favicon-32x32.png">
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: "/openapi.json",
                dom_id: '#swagger-ui',
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIBundle.SwaggerUIStandalonePreset
                ],
                layout: "BaseLayout"
            });
        };
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}
