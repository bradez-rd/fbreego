# Arquitectura del Proyecto en Go — fbreego

## 1. Visión General

El proyecto migrado se estructurará bajo el módulo oficial **`github.com/bradez-rd/fbreego`** utilizando la versión más reciente de Go. La arquitectura sigue los principios de diseño de software limpio (*Clean Architecture*) y las convenciones estándar de la comunidad Go (*Standard Go Project Layout*), garantizando una separación nítida entre lógica de dominio, protocolos de comunicación, persistencia y exposición HTTP/WebSocket.

---

## 2. Estructura de Directorios Propuesta

```text
fbreego/
├── .gitignore
├── go.mod
├── go.sum
├── README.md
├── cmd/
│   └── fbreego/
│       └── main.go                     # Punto de entrada CLI del microservicio
├── pkg/
│   ├── core/                           # Capa fundacional
│   │   ├── types/                      # Estructuras y modelos de dominio (DataFB, DTOs)
│   │   ├── session/                    # dataGetHome y extracción de tokens HTML
│   │   ├── auth/                       # FB4A Login Android y algoritmo 2FA TOTP RFC 6238
│   │   ├── storage/                    # Interfaces de almacenamiento (File, Env, Memory)
│   │   ├── httputil/                   # Cliente HTTP robusto con Keep-Alive y proxy
│   │   ├── permissions/                # Control de permisos POSIX y ACLs Windows
│   │   ├── console/                    # Soporte de terminal UTF-8 segura
│   │   └── utils/                      # Parsers, generadores de IDs, FormAll, Base36
│   ├── messaging/                      # Capa de mensajería Messenger
│   │   ├── send/                       # Envío de texto y adjuntos regulares
│   │   ├── attachments/                # Carga multipart/form-data
│   │   ├── reactions/                  # Mutación GraphQL para añadir/quitar reacciones
│   │   ├── unsend/                     # Endpoint de cancelación de mensajes
│   │   ├── edit/                       # Tareas LightSpeed para edición de texto
│   │   ├── theme/                      # Consulta y aplicación de temas gráficos
│   │   ├── notes/                      # CRUD de notas de 24 horas (Messenger Notes)
│   │   ├── requests/                   # Bandeja de solicitudes de mensajes
│   │   ├── listener/                   # Cliente MQTT TLS edge-chat y control de secuencias
│   │   └── e2ee/                       # Protocolo cifrado E2EE nativo o puente IPC
│   ├── features/                       # Capa de red social y administración
│   │   ├── thread/                     # Administradores, nombres, emojis, apodos y batch
│   │   └── social/                     # Post, bio, bloqueo, marketplace, búsqueda, unfriend
│   └── service/                        # Capa de microservicio REST y tiempo real
│       ├── api/                        # Handlers HTTP, middlewares de auth y rutas REST
│       ├── sessions/                   # Registro concurrente y almacén cifrado AES-256-GCM
│       ├── events/                     # RealtimeEventHub (WebSocket) y SessionListenerManager
│       ├── dispatcher/                 # Despachador asíncrono de webhooks con firma HMAC y SSRF
│       ├── webhook/                    # Almacén de configuración de webhook
│       └── media/                      # Almacén temporal de medios con hash SHA-256
├── web/
│   ├── portal.html                     # SPA administrativa incrustada con //go:embed
│   └── embed.go
└── testutil/
    ├── mocks/                          # Servidores mock de Facebook y GraphQL
    └── fixtures/                       # Respuestas HTML y JSON de muestra
```

---

## 3. Patrones y Reglas de Arquitectura Go

### 3.1 Propagación Obligatoria de `context.Context`
Todas las funciones que realicen operaciones bloqueantes, de red, I/O o procesamiento asíncrono deben recibir `ctx context.Context` como su primer argumento:

```go
func (s *SessionExtractor) ExtractSession(ctx context.Context, cookies string) (*types.DataFB, error)
func (c *Client) SendTextMessage(ctx context.Context, threadID string, text string) (*types.SendResult, error)
```

Esto permite:
- Cancelación inmediata si el cliente HTTP desconecta.
- Cumplimiento estricto de tiempos límite (`context.WithTimeout`).
- Rastreo distribuido o inyección de identificadores de traza (`traceId`).

### 3.2 Manejo Explícito y Jerárquico de Errores
Se erradican los `panic` en el código de producción. Los errores se definen como tipos o centinelas:

```go
var (
    ErrSessionInvalid   = errors.New("fbreego: invalid session or missing tokens")
    ErrTwoFactorRequired = errors.New("fbreego: two-factor authentication required")
    ErrRateLimited      = errors.New("fbreego: request throttled by facebook")
    ErrSSRFDetected     = errors.New("fbreego: target webhook address is blocked by SSRF policy")
)
```

Para errores que transportan metadatos (como respuestas de Facebook con código y subcódigo de error), se definen structs que implementan la interfaz `error`:

```go
type FacebookAPIError struct {
    Code         int    `json:"code"`
    Subcode      int    `json:"subcode,omitempty"`
    Title        string `json:"title"`
    Description  string `json:"description"`
    FBTraceID    string `json:"fbtrace_id,omitempty"`
    HTTPStatus   int    `json:"http_status"`
}

func (e *FacebookAPIError) Error() string {
    return fmt.Sprintf("facebook api error [%d:%d]: %s - %s", e.Code, e.Subcode, e.Title, e.Description)
}
```

### 3.3 Gestión de Concurrencia y Prevención de Fugas de Goroutines
1. **Goroutines acotadas:** Ninguna goroutine debe iniciarse con `go func()` sin un canal de salida o un contexto que señale cuándo debe terminar.
2. **Coordinación de parada:** Se utiliza `sync.WaitGroup` o canales `done <-chan struct{}` para asegurar apagados limpios (*graceful shutdown*).
3. **Control de concurrencia y Mutexes:** Para los mapas de clientes WebSocket y registros de sesiones se emplea `sync.RWMutex`, minimizando la contención de bloqueos durante lecturas concurrentes.

### 3.4 Serialización Estricta y Sanitización JSON
Para las respuestas de Facebook que contienen el prefijo `for (;;);`, se implementa un lector/transformador en Go:

```go
func CleanFacebookJSON(data []byte) []byte {
    trimmed := bytes.TrimSpace(data)
    prefix := []byte("for (;;);")
    if bytes.HasPrefix(trimmed, prefix) {
        return bytes.TrimSpace(trimmed[len(prefix):])
    }
    return trimmed
}
```

---

## 4. Dependencias Externas Justificadas

Para preservar la pureza, ligereza y auditabilidad del código en Go, se limitan las dependencias a paquetes esenciales y de amplio soporte:

| Paquete Go | Justificación |
|---|---|
| Estándar `net/http`, `crypto/*`, `context` | Toda la base de red, TLS, AES-GCM, HMAC y hashing. |
| `github.com/go-chi/chi/v5` o `net/http` (Go 1.22+ mux) | Enrutamiento REST de alto rendimiento compatible 100 % con `http.Handler`. |
| `github.com/coder/websocket` (anteriormente `nhooyr.io/websocket`) | Implementación moderna, segura e idiomática de WebSocket para Go. |
| `github.com/eclipse/paho.golang` | Protocolo MQTT v3.1.1 para conexión de tiempo real con Facebook edge-chat. |
| `golang.org/x/sys` | Manejo de llamadas de sistema multiplataforma (permisos de archivos en Windows/Linux). |

---

## 5. Estrategia del Puente E2EE: ¿Subproceso o Integración Nativa?

El análisis exhaustivo del repositorio original demostró que el puente de cifrado utilizado por `fbchat_v2` en Python (`fbchat-bridge-e2ee`) ya era un binario compilado en Go derivado del ecosistema `mautrix-meta` / `messagix`.

En la arquitectura Go de `fbreego`, se establecen dos modos de operación configurables mediante la variable `FBCHAT_E2EE_MODE`:
1. **Modo Nativo (Recomendado):** Se compila e integra la lógica del protocolo en el mismo proceso Go, ofreciendo rendimiento óptimo, menor huella de memoria y cero latencia IPC.
2. **Modo Compatibilidad (Subproceso):** Si el usuario proporciona un binario externo mediante `FBCHAT_E2EE_BIN`, el módulo `pkg/messaging/e2ee` mantiene la compatibilidad con el protocolo JSON-RPC v1 sobre stdin/stdout idéntico al código Python.
