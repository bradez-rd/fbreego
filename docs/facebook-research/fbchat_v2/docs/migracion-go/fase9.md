# Fase 9 — Integración del Servicio REST, Despachador de Webhooks y Portal Web

### A. Información General
- **Identificador:** FASE-09
- **Nombre de Fase:** Microservicio HTTP REST, Despacho Asíncrono de Webhooks con Protección SSRF y Portal SPA
- **Objetivo:** Implementar el microservicio completo en Go reemplazando FastAPI, integrando los endpoints REST, autenticación HTTP Basic, generación de tickets y streaming WebSocket, cola asíncrona de webhooks con firma HMAC-SHA256 y servido del portal de administración `portal.html` incrustado en el binario.
- **Prioridad:** Alta (P1 - Crítica)
- **Dependencias:** FASE-04, FASE-05, FASE-06, FASE-07, FASE-08.
- **Módulos originales involucrados:**
  - `fbchat_v2/service.py`
  - `fbchat_v2/service_dispatcher.py`
  - `fbchat_v2/service_webhook.py`
  - `fbchat_v2/portal.html`
- **Archivos Go previstos:**
  - `pkg/service/api/server.go`
  - `pkg/service/api/routes.go`
  - `pkg/service/api/handlers_session.go`
  - `pkg/service/api/handlers_messages.go`
  - `pkg/service/api/handlers_social.go`
  - `pkg/service/api/handlers_webhook.go`
  - `pkg/service/api/handlers_media.go`
  - `pkg/service/api/middleware.go`
  - `pkg/service/dispatcher/dispatcher.go`
  - `pkg/service/dispatcher/ssrf.go`
  - `pkg/service/webhook/config_store.go`
  - `web/embed.go`
- **Estado inicial:** `PENDIENTE`

---

### B. Análisis del Código Original
- **Archivos y símbolos relevantes:**
  - `service.py`:
    - Middleware HTTP Basic (`FBCHAT_API_USERNAME`, `FBCHAT_API_PASSWORD`, `FBCHAT_API_AUTH_REQUIRED`).
    - Manejador de errores estándar `_success()` y `_error()` con `traceId` y códigos consistentes (`SUCCESS`, `UNAUTHORIZED`, `INVALID_REQUEST`, `SESSION_NOT_FOUND`, `REQUEST_FAILED`).
    - Emisión y consumo de tickets WebSocket efímeros (`/api/v1/websocket-ticket` con TTL de 30 segundos).
    - Endpoint WebSocket `/ws` con autenticación por ticket, token o basic auth.
    - Diagnóstico del sistema y registro circular en memoria de los últimos 100 logs de auditoría.
  - `service_dispatcher.py`:
    - Cola asíncrona (`asyncio.Queue`) que procesa entregas de eventos en segundo plano.
    - Algoritmo `_is_ssrf_risk`: resuelve DNS y bloquea IPs en rangos privados RFC 1918, loopback, link-local e instancias cloud (`169.254.169.254`).
    - Firma de payloads mediante cabecera `X-Hub-Signature-256: sha256=<hex_hmac>`.
    - Método `test_probe()` para validar URLs destino antes de guardar.
  - `portal.html`: Aplicación web SPA completa en dark mode con conexión directa al WebSocket.
- **Riesgos específicos:**
  - Divergencias en el formato exacto de respuestas de error JSON que rompan clientes existentes.
  - Bloqueos de red al realizar peticiones de Webhook salientes hacia servidores lentos.

---

### C. Plan de Implementación
- **Arquitectura propuesta:** Servidor HTTP Go estándar con enrutador `chi` o `net/http`, worker pool desacoplado para webhooks y `embed.FS` para servir `portal.html`.
- **Interfaces y estructuras Go:**
  ```go
  type Server struct {
      router     http.Handler
      registry   *sessions.Registry
      eventHub   *events.RealtimeHub
      dispatcher *dispatcher.Dispatcher
      mediaStore *media.Storage
      auditLogs  *AuditBuffer
      tickets    *TicketStore
  }

  func (s *Server) ListenAndServe(ctx context.Context, addr string) error
  ```

---

### D. Tareas Ejecutables
- [ ] Implementar middleware de autenticación HTTP Basic con comparación de tiempo constante (`crypto/subtle.ConstantTimeCompare`).
- [ ] Implementar el almacén en memoria de tickets WebSocket efímeros con expiración a los 30 segundos.
- [ ] Implementar el despachador de webhooks en Go (`chan WebhookTask` con pool de goroutines).
- [ ] Implementar el validador estricto anti-SSRF (`pkg/service/dispatcher/ssrf.go`).
- [ ] Implementar la firma HMAC-SHA256 para eventos salientes de webhook (`X-Hub-Signature-256`).
- [ ] Implementar todos los endpoints REST de sesión, mensajería, hilos, redes y medios.
- [ ] Incrustar `portal.html` mediante `//go:embed` y servirlo en la raíz `/` con autenticación protegida.
- [ ] Implementar parada limpia (*graceful shutdown*) esperando que el despachador de webhooks y los listeners terminen ordenadamente.

---

### E. Matriz de Equivalencia
| Componente original | Referencia de origen | Equivalente Go | Prueba | Estado |
|---|---|---|---|---|
| `create_app` | `service.py:create_app` | `pkg/service/api.NewServer()` | Test de arranque HTTP | PENDIENTE |
| `require_auth` | `service.py:require_auth` | `pkg/service/api.BasicAuthMiddleware()` | Test de credenciales 401/200 | PENDIENTE |
| `issue_websocket_ticket` | `service.py:issue_websocket_ticket` | `pkg/service/api.IssueTicket()` | Test de ticket y expiración | PENDIENTE |
| `_is_ssrf_risk` | `service_dispatcher.py:_is_ssrf_risk` | `pkg/service/dispatcher.IsSSRFRisk()` | Test con IPs privadas/públicas | PENDIENTE |
| `WebhookDispatcher` | `service_dispatcher.py:WebhookDispatcher` | `pkg/service/dispatcher.Dispatcher` | Test de entrega y firma HMAC | PENDIENTE |

---

### F. Pruebas y Aceptación
- Comprobación de que todos los endpoints REST devuelvan exactamente los mismos códigos de estado y esquemas JSON que FastAPI.
- Prueba de SSRF: verificar que intentos de enviar webhooks a `127.0.0.1`, `10.0.0.1`, `169.254.169.254` o `localhost` sean bloqueados de inmediato con error 400.
- Prueba del endpoint `/ws`: verificar autenticación exitosa mediante ticket efímero de un solo uso.

---

### G. Riesgos y Dependencias
- Manejo de tiempos de espera en el cliente HTTP del despachador de webhooks (`timeout: 10s`).

---

### H. Definición de Terminado
La fase se considerará terminada cuando el servidor HTTP en Go responda a todas las rutas de la API, entregue el portal web en `/`, procese conexiones WebSocket y despache webhooks firmados con protección SSRF.
