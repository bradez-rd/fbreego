package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// OpenAPISchema devuelve el esquema OpenAPI 3.0.3 de la API REST de fbreego.
func OpenAPISchema() map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "fbchat-provider",
			"description": "Microservicio de alto rendimiento en Go para Facebook Messenger (fbchat_v2)",
			"version":     Version,
		},
		"paths": map[string]any{
			"/healthz": map[string]any{
				"get": map[string]any{
					"summary": "Healthcheck endpoint",
					"responses": map[string]any{
						"200": map[string]any{"description": "Service is alive"},
					},
				},
			},
			"/readyz": map[string]any{
				"get": map[string]any{
					"summary": "Readiness endpoint",
					"responses": map[string]any{
						"200": map[string]any{"description": "Service is ready"},
						"503": map[string]any{"description": "Service not ready"},
					},
				},
			},
			"/api/v1/sessions": map[string]any{
				"get": map[string]any{
					"summary": "List managed sessions",
				},
				"post": map[string]any{
					"summary": "Create and register a new session",
				},
			},
			"/api/v1/webhook": map[string]any{
				"get": map[string]any{
					"summary": "Get current webhook configuration",
				},
				"put": map[string]any{
					"summary": "Update webhook configuration",
				},
			},
			"/api/v1/media/upload": map[string]any{
				"post": map[string]any{
					"summary": "Upload binary or multipart media file",
				},
			},
		},
	}
}

// ServeOpenAPI sirve el JSON del esquema OpenAPI.
func (s *Server) ServeOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(OpenAPISchema())
}

// ServeSwaggerUI sirve una interfaz HTML Swagger UI interactiva.
func (s *Server) ServeSwaggerUI(w http.ResponseWriter, r *http.Request) {
	html := fmt.Sprintf(`<!DOCTYPE html>
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
</html>`)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}
