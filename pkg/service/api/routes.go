package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/bradez-rd/fbreego/web"
)

// Routes configura y devuelve el enrutador HTTP de Chi con todas las rutas y middlewares.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(CORSMiddleware)

	// Endpoints públicos de observabilidad
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		JSONResponse(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": ServiceName,
			"version": Version,
		})
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		report := s.ReadinessReport()
		code := http.StatusOK
		if report["status"] != "ready" {
			code = http.StatusServiceUnavailable
		}
		JSONResponse(w, code, report)
	})

	// Documentación OpenAPI y Swagger UI
	r.Get("/openapi.json", s.ServeOpenAPI)
	r.Get("/docs", s.ServeSwaggerUI)

	// Endpoint WebSocket con autenticación por ticket o credenciales
	r.Get("/ws", s.handleWebSocketStream)

	// Grupo de rutas protegidas
	r.Group(func(pr chi.Router) {
		pr.Use(BasicAuthMiddleware)

		// Portal administrativo SPA
		pr.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(web.PortalHTML)
		})

		// Información y diagnóstico de la aplicación
		pr.Get("/api/v1/app/info", func(w http.ResponseWriter, r *http.Request) {
			JSONSuccess(w, "App info retrieved.", map[string]any{
				"service": ServiceName,
				"version": Version,
				"auth": map[string]any{
					"required": os.Getenv("FBCHAT_API_AUTH_REQUIRED") != "0",
				},
				"bridge": map[string]any{
					"ready": true,
				},
			}, http.StatusOK)
		})

		pr.Get("/api/v1/app/status", func(w http.ResponseWriter, r *http.Request) {
			total := s.Registry.Count()
			active := 0
			for _, item := range s.Registry.ListPublic("", false) {
				if item["status"] == "active" {
					active++
				}
			}
			JSONSuccess(w, "App status retrieved.", map[string]any{
				"totalSessions":  total,
				"activeSessions": active,
				"wsClients":      s.EventHub.ClientCount(),
			}, http.StatusOK)
		})

		pr.Get("/api/v1/diagnostics", func(w http.ResponseWriter, r *http.Request) {
			s.auditMu.Lock()
			logsCopy := make([]AuditLogEntry, len(s.auditLogs))
			copy(logsCopy, s.auditLogs)
			s.auditMu.Unlock()

			JSONSuccess(w, "Diagnostics retrieved.", map[string]any{
				"service":    ServiceName,
				"version":    Version,
				"auditLogs":  logsCopy,
				"dispatcher": s.Dispatcher.Stats(),
			}, http.StatusOK)
		})

		// Emisión de ticket efímero para WebSocket
		pr.Post("/api/v1/websocket-ticket", func(w http.ResponseWriter, r *http.Request) {
			ticket := s.IssueWebSocketTicket()
			var body map[string]any
			if r.Body != nil {
				_ = json.NewDecoder(r.Body).Decode(&body)
			}
			sessionID, _ := body["sessionId"].(string)
			if sessionID == "" {
				sessionID = r.URL.Query().Get("sessionId")
			}

			JSONSuccess(w, "One-time WebSocket ticket created.", map[string]any{
				"ticket":    ticket,
				"expiresIn": WebSocketTicketTTLSecs,
				"sessionId": sessionID,
			}, http.StatusCreated)
		})

		// Sesiones
		pr.Get("/api/v1/sessions", s.handleListSessions)
		pr.Post("/api/v1/sessions", s.handleCreateSession)
		pr.Get("/api/v1/sessions/{sessionId}/status", s.handleGetSessionStatus)
		pr.Post("/api/v1/sessions/{sessionId}/validate", s.handleValidateSession)
		pr.Delete("/api/v1/sessions/{sessionId}", s.handleDeleteSession)

		// Listeners de sesión
		pr.Post("/api/v1/sessions/{sessionId}/listen/start", s.handleStartListening)
		pr.Post("/api/v1/sessions/{sessionId}/listen/stop", s.handleStopListening)
		pr.Get("/api/v1/sessions/{sessionId}/listen/status", s.handleGetListenStatus)

		// Conversaciones / Threads
		pr.Get("/api/v1/sessions/{sessionId}/threads", s.handleListThreads)

		// Mensajería
		pr.Post("/api/v1/sessions/{sessionId}/messages/text", s.handleSendTextMessage)
		pr.Post("/api/v1/sessions/{sessionId}/messages/reaction", s.handleReactMessage)
		pr.Post("/api/v1/sessions/{sessionId}/messages/unsend", s.handleUnsendMessage)
		pr.Post("/api/v1/sessions/{sessionId}/messages/media", s.handleSendMediaMessage)

		// Social & Privacidad
		pr.Get("/api/v1/sessions/{sessionId}/users/{userId}", s.handleGetUserInfo)
		pr.Get("/api/v1/sessions/{sessionId}/search", s.handleSearchUsers)
		pr.Post("/api/v1/sessions/{sessionId}/privacy/block", s.handleBlockUser)
		pr.Get("/api/v1/sessions/{sessionId}/marketplace/{itemId}", s.handleGetMarketplaceItem)

		// Webhook
		pr.Get("/api/v1/webhook", s.handleGetWebhookConfig)
		pr.Put("/api/v1/webhook", s.handleUpdateWebhookConfig)
		pr.Delete("/api/v1/webhook", s.handleClearWebhookConfig)
		pr.Post("/api/v1/webhook/test", s.handleTestWebhook)

		// Medios
		pr.Post("/api/v1/media/upload", s.handleUploadMedia)
		pr.Get("/api/v1/media", s.handleListMedia)
		pr.Get("/api/v1/media/{mediaId}", s.handleGetMediaFile)
		pr.Get("/api/v1/media/{mediaId}/info", s.handleGetMediaInfo)
		pr.Delete("/api/v1/media/{mediaId}", s.handleDeleteMedia)
	})

	return r
}

func (s *Server) handleWebSocketStream(w http.ResponseWriter, r *http.Request) {
	reqAuth := os.Getenv("FBCHAT_API_AUTH_REQUIRED") != "0"
	ticket := r.URL.Query().Get("ticket")
	token := r.URL.Query().Get("token")
	sessionID := r.URL.Query().Get("sessionId")

	if reqAuth {
		authenticated := false
		if s.ConsumeWebSocketTicket(ticket) {
			authenticated = true
		} else if token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(os.Getenv("FBCHAT_API_PASSWORD"))) == 1 {
			authenticated = true
		} else {
			user, pass, ok := r.BasicAuth()
			if ok && subtle.ConstantTimeCompare([]byte(user), []byte(os.Getenv("FBCHAT_API_USERNAME"))) == 1 &&
				subtle.ConstantTimeCompare([]byte(pass), []byte(os.Getenv("FBCHAT_API_PASSWORD"))) == 1 {
				authenticated = true
			}
		}

		if !authenticated {
			JSONError(w, "UNAUTHORIZED", "Invalid WebSocket credentials or expired ticket.", http.StatusUnauthorized, nil, "")
			return
		}
	}

	opts := &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	}
	ws, err := websocket.Accept(w, r, opts)
	if err != nil {
		return
	}
	defer ws.Close(websocket.StatusNormalClosure, "disconnect")

	sub := s.EventHub.Register(ws, sessionID)
	defer s.EventHub.Unregister(sub)

	// Lanzar loop de envío de mensajes hacia el cliente
	go sub.PumpWrites(r.Context())

	// Loop de lectura (para mantener viva la conexión o procesar pings)
	for {
		_, _, err := ws.Read(r.Context())
		if err != nil {
			break
		}
	}
}
