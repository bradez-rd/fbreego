package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/bradez-rd/fbreego/pkg/service/traffic"
	"github.com/bradez-rd/fbreego/web"
)

// Routes configura y devuelve el enrutador HTTP de Chi con todas las rutas y middlewares.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(CORSMiddleware)
	if s.TrafficEnabled {
		r.Use(traffic.InboundTrafficMiddleware(s.TrafficStore))
	}

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
			auth := currentAuthConfiguration()
			storageType := "memory"
			if s.Registry != nil && s.Registry.HasEncryptedStorage() {
				storageType = "encrypted"
			}
			JSONSuccess(w, "App info retrieved.", map[string]any{
				"service":        ServiceName,
				"version":        Version,
				"apiVersion":     "v1",
				"sessionStorage": storageType,
				"auth": map[string]any{
					"required":   auth.Required,
					"configured": auth.Configured,
				},
				"bridge": map[string]any{
					"ready": false,
				},
				"capabilities": []string{
					"sessions",
					"session-validation",
					"session-listeners",
					"global-webhook-config",
					"webhook-dispatcher",
					"websocket-events",
					"media-upload",
					"media-send",
					"threads",
					"text-messages",
					"reactions",
					"unsend",
					"user-info",
					"search",
					"blocking",
					"marketplace-read",
					"traffic-inspection",
				},
				"capabilityMatrix": map[string]any{
					"messenger.send": map[string]string{
						"status":      "supported",
						"description": "Send text & replies to users and groups",
					},
					"messenger.receive": map[string]string{
						"status":      "supported",
						"description": "Webhook ingestion for messages & sessions",
					},
					"messenger.threads": map[string]string{
						"status":      "supported",
						"description": "Sanitized conversation listing",
					},
					"messenger.reactions": map[string]string{
						"status":      "supported",
						"description": "Add and remove emoji reactions",
					},
					"messenger.unsend": map[string]string{
						"status":      "supported",
						"description": "Revoke sent messages",
					},
					"messenger.users": map[string]string{
						"status":      "supported",
						"description": "Lookup sanitized profile",
					},
					"messenger.blocking": map[string]string{
						"status":      "supported",
						"description": "Block and unblock Facebook contacts",
					},
					"messenger.attachments": map[string]string{
						"status":      "supported",
						"description": "Upload and dispatch staged multimedia",
					},
					"marketplace.read": map[string]string{
						"status":      "supported",
						"description": "Fetch public Marketplace listings",
					},
					"traffic.inspector": map[string]string{
						"status":      "supported",
						"description": "Realtime HTTP traffic debugging & cURL export",
					},
				},
			}, http.StatusOK)
		})

		pr.Get("/api/v1/app/status", func(w http.ResponseWriter, r *http.Request) {
			total := s.Registry.Count()
			active := 0
			for _, item := range s.Registry.ListPublic("", false) {
				st, _ := item["status"].(string)
				if st == "active" || st == "connected" {
					active++
				}
			}
			rep := s.ReadinessReport()
			readinessStatus, _ := rep["status"].(string)
			if readinessStatus == "" {
				readinessStatus = "ready"
			}
			statusStr := "idle"
			if total > 0 {
				statusStr = "connected"
			}
			var dispStats map[string]any
			if s.Dispatcher != nil {
				dispStats = s.Dispatcher.Stats()
			}
			var hubMetrics map[string]any
			if s.EventHub != nil {
				hubMetrics = s.EventHub.Metrics()
			}
			clientCount := 0
			if s.EventHub != nil {
				clientCount = s.EventHub.ClientCount()
			}
			JSONSuccess(w, "App status retrieved.", map[string]any{
				"status":         statusStr,
				"service":        ServiceName,
				"version":        Version,
				"totalSessions":  total,
				"activeSessions": active,
				"readiness":      readinessStatus,
				"dispatcher":     dispStats,
				"eventHub":       hubMetrics,
				"wsClients":      clientCount,
			}, http.StatusOK)
		})

		pr.Get("/api/v1/diagnostics", func(w http.ResponseWriter, r *http.Request) {
			s.auditMu.Lock()
			logsCopy := make([]AuditLogEntry, len(s.auditLogs))
			copy(logsCopy, s.auditLogs)
			s.auditMu.Unlock()

			var dispStats map[string]any
			if s.Dispatcher != nil {
				dispStats = s.Dispatcher.Stats()
			}
			var hubMetrics map[string]any
			if s.EventHub != nil {
				hubMetrics = s.EventHub.Metrics()
			}

			JSONSuccess(w, "Diagnostics retrieved.", map[string]any{
				"service":         ServiceName,
				"version":         Version,
				"readiness":       s.ReadinessReport(),
				"sessionsCount":   s.Registry.Count(),
				"auditLogs":       logsCopy,
				"dispatcher":      dispStats,
				"dispatcherStats": dispStats,
				"eventHub":        hubMetrics,
			}, http.StatusOK)
		})

		// Emisión de ticket efímero para WebSocket
		pr.Post("/api/v1/websocket-ticket", func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			if r.Body != nil {
				_ = json.NewDecoder(r.Body).Decode(&body)
			}
			sessionID, _ := body["sessionId"].(string)
			if sessionID == "" {
				sessionID = r.URL.Query().Get("sessionId")
			}
			ticket := s.IssueWebSocketTicket(sessionID)

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

		// Inspector de Tráfico HTTP (Facebook y API)
		pr.Get("/api/v1/traffic", s.handleListTraffic)
		pr.Get("/api/v1/traffic/stats", s.handleGetTrafficStats)
		pr.Get("/api/v1/traffic/{id}", s.handleGetTrafficDetail)
		pr.Delete("/api/v1/traffic", s.handleClearTraffic)

		// Suite de Tests E2E entre Sesiones y APIs
		if envFlag("FBCHAT_ENABLE_E2E_TEST_ENDPOINT", false) {
			pr.Post("/api/v1/tests/e2e", s.handleRunE2ETests)
		}
	})

	return r
}

func (s *Server) handleWebSocketStream(w http.ResponseWriter, r *http.Request) {
	auth := currentAuthConfiguration()
	ticket := r.URL.Query().Get("ticket")
	sessionID := r.URL.Query().Get("sessionId")

	if auth.Required {
		if !auth.Configured {
			JSONError(w, "AUTH_NOT_CONFIGURED", "API authentication is required but credentials are not configured.", http.StatusServiceUnavailable, nil, "")
			return
		}
		authenticated := false
		if s.ConsumeWebSocketTicket(ticket, sessionID) {
			authenticated = true
		} else {
			user, pass, ok := r.BasicAuth()
			if ok && subtle.ConstantTimeCompare([]byte(user), []byte(auth.Username)) == 1 &&
				subtle.ConstantTimeCompare([]byte(pass), []byte(auth.Password)) == 1 {
				authenticated = true
			}
		}

		if !authenticated {
			JSONError(w, "UNAUTHORIZED", "Invalid WebSocket credentials or expired ticket.", http.StatusUnauthorized, nil, "")
			return
		}
	}

	var originPatterns []string
	for _, origin := range strings.Split(strings.TrimSpace(os.Getenv("FBCHAT_API_WS_ORIGINS")), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			originPatterns = append(originPatterns, origin)
		}
	}
	opts := &websocket.AcceptOptions{OriginPatterns: originPatterns}
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
