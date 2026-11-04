# Inventario Funcional Exhaustivo — fbchat_v2

Este inventario documenta detalladamente todos los módulos, archivos, clases, funciones y endpoints que componen el repositorio **`facebook-research/fbchat_v2`** (versión 2.3.3).

---

## 1. Módulo Raíz y Microservicio Provider

### 1.1 `__init__.py`
- **Ruta de origen:** `fbchat_v2/__init__.py`
- **Propósito:** Espacio de nombres público del paquete `fbchat-v2`. Exporta versiones y funciones canónicas.
- **Símbolos exportados:** `__version__`, `__author__`, `__license__`, `dataGetHome`, `EnvSessionStorage`, `FileSessionStorage`, `listeningEvent`, `listeningE2EEEvent`, `BridgeActions`, `SendAPI`, `E2EESendAPI`, `sendingE2EEEvent`, `editMessage`, `changeTheme`, `createNotes`.
- **Estrategia Go:** Paquete raíz o reexportación en `pkg/fbreego` como API unificada para desarrolladores.

### 1.2 `__main__.py`
- **Ruta de origen:** `fbchat_v2/__main__.py`
- **Propósito:** Punto de entrada CLI para ejecutar el microservicio directamente (`python -m fbchat_v2`).
- **Símbolos:** Invocación a `fbchat_v2.service.main()`.
- **Estrategia Go:** `cmd/fbreego/main.go`.

### 1.3 `portal.html`
- **Ruta de origen:** `fbchat_v2/portal.html` (68 KB)
- **Propósito:** Interfaz gráfica SPA integrada (Dark Theme) para administración de sesiones, monitor en tiempo real con WebSocket, envío de mensajes y diagnóstico.
- **Estrategia Go:** Incrustación en el binario Go mediante la directiva estándar `//go:embed portal.html` de Go 1.16+.

### 1.4 `service.py`
- **Ruta de origen:** `fbchat_v2/service.py` (1,566 líneas)
- **Propósito:** Servidor de microservicio REST y streaming WebSocket FastAPI.
- **Funcionalidades y Endpoints:**
  - `readiness_report()` / `GET /readyz`: Chequeo de salud del servicio, bridge y credenciales.
  - `GET /healthz`: Chequeo liviano de actividad (`{"status": "ok"}`).
  - `GET /`: Sirve el portal de administración web (`portal.html`).
  - `GET /openapi.json` y `GET /docs`: Especificación OpenAPI y Swagger UI.
  - `POST /api/v1/websocket-ticket`: Genera ticket efímero (30 s TTL) con `crypto/rand` para autenticación WebSocket.
  - `WEBSOCKET /ws`: Stream bidireccional en tiempo real para eventos de sesiones (mensajes, estados, logs).
  - `GET /api/v1/app/info`: Información de runtime, versiones y chequeo de puente E2EE.
  - `GET /api/v1/app/status`: Conteo total de sesiones, activas y en error.
  - `GET /api/v1/diagnostics`: Diagnóstico detallado del sistema y registro de auditoría en memoria (últimos 100 eventos).
  - `GET /api/v1/sessions`: Lista sesiones (con o sin detalle, filtrado por `tenantId`).
  - `POST /api/v1/sessions`: Crea y valida una nueva sesión a partir de lista de cookies.
  - `GET /api/v1/sessions/{id}/status`: Obtiene el estado actual de una sesión.
  - `POST /api/v1/sessions/{id}/validate`: Revalida sesión contra Facebook (`dataGetHome`).
  - `DELETE /api/v1/sessions/{id}`: Elimina la sesión de memoria y almacenamiento persistente.
  - `POST /api/v1/sessions/{id}/listen/start`: Inicia la escucha en tiempo real de la sesión en background.
  - `POST /api/v1/sessions/{id}/listen/stop`: Detiene la escucha en tiempo real.
  - `GET /api/v1/sessions/{id}/listen/status`: Devuelve si el listener está activo.
  - `GET /api/v1/sessions/{id}/threads`: Lista conversaciones y threads de la sesión.
  - `POST /api/v1/sessions/{id}/messages/text`: Envía mensaje de texto a un usuario o grupo.
  - `POST /api/v1/sessions/{id}/messages/reaction`: Agrega o remueve reacción emoji a un mensaje.
  - `POST /api/v1/sessions/{id}/messages/unsend`: Cancela el envío de un mensaje.
  - `POST /api/v1/sessions/{id}/messages/media`: Envía mensaje multimedia con IDs previamente cargados.
  - `GET /api/v1/sessions/{id}/users/{user_id}`: Consulta perfil de un usuario.
  - `GET /api/v1/sessions/{id}/search`: Búsqueda de personas y contactos.
  - `POST /api/v1/sessions/{id}/privacy/block`: Bloquea o desbloquea un usuario.
  - `GET /api/v1/sessions/{id}/marketplace/{item_id}`: Consulta detalles de producto de Marketplace.
  - `GET /api/v1/webhook`: Consulta configuración actual de webhook.
  - `PUT /api/v1/webhook`: Actualiza URL, eventos suscritos y secreto HMAC del webhook.
  - `DELETE /api/v1/webhook`: Borra la configuración del webhook.
  - `POST /api/v1/webhook/test`: Ejecuta prueba de despacho al webhook.
  - `POST /api/v1/media/upload`: Sube archivo binario o base64 a almacenamiento temporal de medios.
  - `GET /api/v1/media`: Lista medios almacenados en caché.
  - `GET /api/v1/media/{media_id}`: Descarga binario del medio.
  - `GET /api/v1/media/{media_id}/info`: Metadatos del medio.
  - `DELETE /api/v1/media/{media_id}`: Elimina medio del disco.
- **Estrategia Go:** Paquete `pkg/service/api`, implementado con router HTTP estándar o `chi`, manejadores modulares y middleware de autenticación HTTP Basic.

### 1.5 `service_sessions.py`
- **Ruta de origen:** `fbchat_v2/service_sessions.py`
- **Clases y Funciones:**
  - `SessionRecord`: Entidad de sesión (`session_id`, `label`, `tenant_id`, `cookies`, `facebook_id`, `status`, `created_at`, `last_activity`, `last_error`).
  - `EncryptedSessionStore`: Almacenamiento cifrado en disco por tenant (cifrado simétrico AES/Fernet).
  - `SessionRegistry`: Registro concurrente en memoria con semáforo y métodos: `create()`, `list_public()`, `validate()`, `delete()`, `record_activity()`, `record_error()`, `get_session_status()`, `count()`, `get()`, `get_provider_data()`.
- **Estrategia Go:** `pkg/service/sessions` con `sync.RWMutex`, cifrado AES-256-GCM y persistencia JSON atómica con permisos restringidos.

### 1.6 `service_events.py`
- **Ruta de origen:** `fbchat_v2/service_events.py`
- **Clases y Funciones:**
  - `RealtimeEventHub`: Hub Pub/Sub en memoria que registra conexiones WebSocket, gestiona desconexiones, emite métricas y despacha eventos a clientes WebSocket y al `WebhookDispatcher`.
  - `SessionListenerManager`: Coordinador del ciclo de vida de los listeners en background de cada sesión. Inicia, monitorea y detiene tareas concurrentes.
- **Estrategia Go:** `pkg/service/events` con canales Go, `sync.Map` para clientes WebSocket activos y `context.CancelFunc` por sesión.

### 1.7 `service_dispatcher.py`
- **Ruta de origen:** `fbchat_v2/service_dispatcher.py`
- **Clases y Funciones:**
  - `WebhookDispatcher`: Cola asíncrona de eventos hacia URLs externas.
  - `_is_ssrf_risk()`: Validación contra ataques SSRF (bloqueo de rangos IP privados 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 127.0.0.0/8, loopback IPv6 `::1`, link-local `169.254.0.0/16` y metadatos de cloud).
  - `_deliver_event()`: Despacho HTTP POST con firma HMAC-SHA256 (`X-Hub-Signature-256`) y política de reintentos.
  - `test_probe()`: Verificación puntual de conectividad hacia una URL destino.
- **Estrategia Go:** `pkg/service/dispatcher` utilizando un canal de eventos como cola (`chan WebhookTask`), un worker pool de goroutines y `net.ParseIP` para validación estricta de SSRF.

### 1.8 `service_webhook.py`
- **Ruta de origen:** `fbchat_v2/service_webhook.py`
- **Clases y Funciones:**
  - `WebhookConfig`: Entidad de configuración (`enabled`, `url`, `events`, `secret`).
  - `WebhookConfigStore`: Almacén cifrado en disco para persistir la configuración global de webhooks.
  - `normalize_webhook_url()`, `validate_webhook_events()`.
- **Estrategia Go:** `pkg/service/webhook` con validación URL en Go (`net/url`), cifrado AES-256-GCM.

### 1.9 `service_media.py`
- **Ruta de origen:** `fbchat_v2/service_media.py`
- **Clases y Funciones:**
  - `MediaRecord`: Metadatos (`media_id`, `file_name`, `mime_type`, `file_size`, `sha256`, `created_at`).
  - `MediaStorage`: Almacén local en disco de archivos temporales con deduplicación por hash SHA-256, saneamiento de nombres de archivo y límites de tamaño.
- **Estrategia Go:** `pkg/service/media` con `io.Copy`, `crypto/sha256` y streaming directo a disco.

### 1.10 `service_provider.py`
- **Ruta de origen:** `fbchat_v2/service_provider.py`
- **Clases y Funciones:**
  - `_parse_lightspeed_data_from_html()`: Extrae estructuras LightSpeed (LS) de la respuesta HTML inicial de Messenger.
  - `_extract_ls_calls()` / `_find_all_ls_requests()`: Parseo de llamadas RPC de LightSpeed embebidas.
  - `fetch_lightspeed_data()`: Realiza petición a `https://www.facebook.com/messages/t/` y extrae hilos y metadatos de sincronización.
  - `ProviderOperations`: Fachada unificada que ejecuta operaciones de alto nivel sobre sesiones (`list_threads`, `list_all_threads`, `send_text`, `react_message`, `unsend_message`, `get_user_info`, `search_users`, `block_user`, `get_marketplace_item`, `send_attachment`).
- **Estrategia Go:** `pkg/service/provider` como orquestador entre las sesiones y las capas de mensajería/features.

---

## 2. Capa Núcleo (`_core`)

### 2.1 `_core/_types.py`
- **Ruta:** `fbchat_v2/_core/_types.py`
- **Estructuras:** `DataFB`, `SuccessResponse`, `ErrorResponse`, `RequestKwargs`, `LoginSuccessPayload`, `LoginErrorPayload`, `AttachmentResult`.
- **Estrategia Go:** `pkg/core/types` con structs fuertemente tipados y serializadores JSON personalizados.

### 2.2 `_core/_utils.py`
- **Ruta:** `fbchat_v2/_core/_utils.py`
- **Funciones:**
  - `Headers()`: Rotación de User-Agents Chrome/Windows/macOS/Linux y generación de cabeceras Client Hints (`Sec-CH-UA`, `Sec-Fetch-*`).
  - `digitToChar()`, `str_base()`: Conversión de base numérica para generar secuencias `__req` en Base36.
  - `parse_cookie_string()`: Descompone cabecera `Cookie: a=1; b=2` en mapa clave-valor.
  - `dataSplit()`: Extracción posicional de fragmentos de texto entre delimitadores dentro de HTML.
  - `formAll()`: Construcción de formulario base Facebook / RelayModern (`__a=1`, `__user`, `__req`, `__rev`, `av`, `fb_dtsg`, `jazoest`, `doc_id`).
  - `clearHTML()`: Sanitización de etiquetas HTML mediante expresiones regulares.
  - `mainRequests()`: Ensambla el diccionario de petición HTTP con cabeceras y cookies.
  - `send_request()`, `send_request_async()`, `send_get_request()`, `send_get_request_async()`: Envoltorios de transporte HTTP.
  - `parse_json_response()`: Decodificación de respuestas JSON de Facebook con remoción obligatoria del prefijo de seguridad `for (;;);`.
  - `post_form_json()`, `post_form_json_async()`: Envía formulario y decodifica JSON limpio.
  - `generate_session_id()`, `generate_client_id()`, `gen_threading_id()`: Generadores pseudoaleatorios de identificadores de sesión, cliente UUIDv4 simulado e IDs de hilo de 64 bits en base a timestamp + bits aleatorios.
  - `json_minimal()`: Serialización JSON compacta sin espacios en blanco (`separators=(',', ':')`).
  - `randStr()`: Generador de cadenas alfanuméricas aleatorias.
- **Estrategia Go:** `pkg/core/utils` con funciones estándar optimizadas en Go (`strings.Builder`, `math/rand/v2` o `crypto/rand`).

### 2.3 `_core/_session.py`
- **Ruta:** `fbchat_v2/_core/_session.py`
- **Funciones:**
  - `dataGetHome(setCookies, storage)`: Realiza petición GET a `https://www.facebook.com/`, extrae tokens obligatorios:
    - `fb_dtsg`: Token CSRF principal (extraído de `DTSGInitialData`).
    - `fb_dtsg_ag`: Token asíncrono secundario.
    - `jazoest`: Hash de control de integridad de formulario.
    - `hash`: Hash de sesión.
    - `sessionID`: ID de sesión web.
    - `FacebookID`: Actor ID del usuario autenticado (valida que sea numérico).
    - `clientRevision`: Versión del cliente web de Facebook.
- **Estrategia Go:** `pkg/core/session` con cliente HTTP configurado y búsqueda eficiente de tokens en el cuerpo HTML.

### 2.4 `_core/_storage.py`
- **Ruta:** `fbchat_v2/_core/_storage.py`
- **Clases:**
  - `SessionStorage` (interfaz base: `load()`, `save()`, `clear()`).
  - `FileSessionStorage`: Almacenamiento en archivo JSON local (`config.json`).
  - `EnvSessionStorage`: Almacenamiento a través de variable de entorno (`FB_COOKIES`).
- **Estrategia Go:** Interfaz `SessionStorage` en `pkg/core/storage` con implementaciones `FileStorage` y `EnvStorage`.

### 2.5 `_core/_http.py`
- **Ruta:** `fbchat_v2/_core/_http.py`
- **Funciones:** Clientes HTTP reutilizables síncronos y asíncronos con `httpx`, gestión de tiempos de espera (`timeout=30s`) y verificación SSL obligatoria.
- **Estrategia Go:** `pkg/core/httputil` con `http.Client`, `http.Transport` configurado con Keep-Alive, HTTP/2 y pool de conexiones.

### 2.6 `_core/_facebookLogin.py`
- **Ruta:** `fbchat_v2/_core/_facebookLogin.py` (545 líneas)
- **Funcionalidades:**
  - Autenticación directa por credenciales mediante el endpoint de la app nativa Android FB4A: `https://b-graph.facebook.com/auth/login`.
  - Soporte de proxy HTTP/HTTPS.
  - Soporte de Two-Factor Authentication (2FA): validación de clave secreta TOTP (`pyotp`), generación de códigos locales de 6 dígitos o aceptación de código directo.
  - Manejo de reintentos y captura de subcódigos de error de 2FA (códigos `1348162`, `1348023`).
  - Extracción y ensamblado de cookies finales (`c_user`, `xs`, `fr`, `datr`, etc.) a partir de la lista `session_cookies` devuelta por Facebook.
- **Estrategia Go:** `pkg/core/auth` con implementación nativa de TOTP en Go (usando RFC 6238 / HMAC-SHA1) y peticiones HTTPS directas con `net/http`.

### 2.7 `_core/_permissions.py`
- **Ruta:** `fbchat_v2/_core/_permissions.py`
- **Funciones:** Asignación de permisos de archivo restrictivos para secretos (`0600` en sistemas POSIX / Unix, y uso de ACLs / `icacls` en Windows).
- **Estrategia Go:** `pkg/core/permissions` con soporte multiplataforma (`os.Chmod` en Unix y `golang.org/x/sys/windows` en Windows).

### 2.8 `_core/_console.py`
- **Ruta:** `fbchat_v2/_core/_console.py`
- **Funciones:** Configuración de consola UTF-8 segura en Windows para evitar errores de codificación al imprimir emojis o caracteres internacionales.
- **Estrategia Go:** `pkg/core/console` configurando `os.Stdout` y `os.Stderr` con soporte nativo de UTF-8 en Go.

---

## 3. Capa de Mensajería (`_messaging`)

### 3.1 `_messaging/_send.py`
- **Ruta:** `fbchat_v2/_messaging/_send.py`
- **Funcionalidades:** Envío de mensajes tradicionales de texto a usuarios individuales o grupos a través del endpoint `https://www.facebook.com/messaging/send/`. Soporte de citas de mensajes (`reply_to_message_id`), adjuntos multimedia y generación de `otid` / `client_thread_id`.
- **Estrategia Go:** `pkg/messaging/send`.

### 3.2 `_messaging/_attachments.py`
- **Ruta:** `fbchat_v2/_messaging/_attachments.py`
- **Funcionalidades:** Carga multipart/form-data de archivos locales o streams hacia `https://upload.facebook.com/ajax/mercury/upload.php`, obteniendo `attachmentID` según el tipo de archivo (imágenes, video, audio, archivos genéricos).
- **Estrategia Go:** `pkg/messaging/attachments` con `mime/multipart` de Go.

### 3.3 `_messaging/_reactions.py`
- **Ruta:** `fbchat_v2/_messaging/_reactions.py`
- **Funcionalidades:** Reaccionar a mensajes de Messenger mediante GraphQL RelayModern (`doc_id` específico). Permite añadir o retirar emojis de reacción.
- **Estrategia Go:** `pkg/messaging/reactions`.

### 3.4 `_messaging/_unsend.py`
- **Ruta:** `fbchat_v2/_messaging/_unsend.py`
- **Funcionalidades:** Cancelar el envío de un mensaje previamente transmitido para todos los miembros de la conversación (`https://www.facebook.com/messaging/unsend_message/`).
- **Estrategia Go:** `pkg/messaging/unsend`.

### 3.5 `_messaging/_editMessage.py`
- **Ruta:** `fbchat_v2/_messaging/_editMessage.py`
- **Funcionalidades:** Edición de mensajes ya enviados mediante la publicación de una tarea LightSpeed (`ls_req`) a través del broker MQTT de Messenger.
- **Estrategia Go:** `pkg/messaging/edit` coordinado con el cliente MQTT.

### 3.6 `_messaging/_changeTheme.py`
- **Ruta:** `fbchat_v2/_messaging/_changeTheme.py`
- **Funcionalidades:** Listar temas gráficos de Messenger disponibles en GraphQL (`listThemes`), buscar tema por nombre (`findTheme`) y aplicar tema a un thread (`changeTheme`).
- **Estrategia Go:** `pkg/messaging/theme`.

### 3.7 `_messaging/_createNotes.py`
- **Ruta:** `fbchat_v2/_messaging/_createNotes.py`
- **Funcionalidades:** Gestión de notas efímeras de 24 horas de Messenger (`checkNote`, `createNote`, `deleteNote`, `recreateNote`) con control de audiencia/privacidad mediante GraphQL.
- **Estrategia Go:** `pkg/messaging/notes`.

### 3.8 `_messaging/_message_requests.py`
- **Ruta:** `fbchat_v2/_messaging/_message_requests.py`
- **Funcionalidades:** Consulta de solicitudes de mensajes pendientes (`inbox de solicitudes / spam`) a través de peticiones GraphQL batch.
- **Estrategia Go:** `pkg/messaging/requests`.

### 3.9 `_messaging/_listening.py`
- **Ruta:** `fbchat_v2/_messaging/_listening.py`
- **Funcionalidades:** Cliente MQTT sobre WebSocket TLS conectado a `edge-chat.facebook.com` / `chat.facebook.com`.
  - Suscripción a tópicos de eventos (`/t_ms`, `/thread_typing`, `/orca_presence`).
  - Control de números de secuencia (`seq_id`) para sincronización fiable sin pérdida de mensajes.
  - Procesamiento de deltas de mensajes entrantes, confirmaciones de entrega y lecturas.
- **Estrategia Go:** `pkg/messaging/listener` con cliente MQTT nativo Go sobre WebSocket TLS.

### 3.10 `_messaging/_listening_e2ee.py` & `_bridge_actions.py` & `_send_e2ee.py`
- **Rutas:**
  - `fbchat_v2/_messaging/_listening_e2ee.py` (2,050 líneas)
  - `fbchat_v2/_messaging/_bridge_actions.py` (359 líneas)
  - `fbchat_v2/_messaging/_send_e2ee.py` (248 líneas)
  - `fbchat_v2/_messaging/_bridge_checksums.py`
- **Funcionalidades:**
  - Protocolo de mensajería cifrada de extremo a extremo (E2EE) de Facebook Messenger (arquitectura Labyrinth / Signal protocol).
  - En la versión Python original, se gestionaba como un subproceso hijo (`_BridgeProcess`) que ejecutaba un binario compilado en Go (`mautrix-meta` / `messagix`) comunicándose por stdin/stdout mediante un protocolo JSON-RPC v1.
  - Acciones soportadas: `editMessage`, `unsendMessage`, `editE2EEMessage`, `unsendE2EEMessage`, `sendTypingIndicator`, `markRead`, `sendE2EETyping`, `sendE2EEAudio`, `sendE2EEImage`, `downloadMedia`, `downloadE2EEMedia`.
  - Redacción estricta de credenciales en logs (`_redact_bridge_text`).
- **Estrategia Go:** **Ventaja Arquitectónica Crítica:** Dado que el puente original es un módulo Go de `mautrix-meta`, en `fbreego` se puede integrar de forma nativa como paquete Go en el mismo espacio de proceso, eliminando la sobrecarga de subprocesos, tuberías y deserializaciones IPC innecesarias, mientras se mantiene un adaptador de compatibilidad para la API de acciones.

---

## 4. Capa de Funcionalidades de Red Social y Threads (`_features`)

### 4.1 Submódulo `_features/_thread/`
1. **`_addAdmin.py`:** Añadir o remover administradores en conversaciones grupales de Messenger.
2. **`_all_thread_data.py`:** Extracción completa de metadatos de hilos de chat mediante GraphQL Batch (`threads`, `message_count`, participantes, apodos, configuración de notificaciones).
3. **`_changeEmoji.py`:** Modificar el emoji por defecto de una conversación grupal.
4. **`_changeNameThread.py`:** Renombrar el título de un grupo de Messenger.
5. **`_changeNickname.py`:** Asignar o eliminar apodos de usuarios dentro de un hilo de conversación.

### 4.2 Submódulo `_features/_facebook/`
1. **`_archivePost.py`:** Archivar una publicación del muro personal.
2. **`_blocking.py`:** Bloquear o desbloquear usuarios en Facebook.
3. **`_changeBio.py`:** Actualizar el texto de la biografía pública del perfil.
4. **`_createPost.py`:** Crear publicaciones en el muro de Facebook (texto y enlaces).
5. **`_deletePost.py`:** Eliminar publicaciones del muro.
6. **`_get_user_info.py`:** Consultar información pública y estructurada del perfil de un usuario.
7. **`_marketplace.py`:** Crear listados de venta de productos y consultar detalles de productos en Facebook Marketplace.
8. **`_notification.py`:** Consultar listado de notificaciones del usuario.
9. **`_professional.py`:** Activar o desactivar el modo profesional en perfiles de Facebook.
10. **`_reactionPost.py`:** Reaccionar (Like, Love, Care, Haha, Wow, Sad, Angry) o des-reaccionar a publicaciones del feed.
11. **`_registerOnProfile.py`:** Gestión y registro de perfiles adicionales asociados a la cuenta.
12. **`_search.py`:** Búsqueda global de usuarios, personas y perfiles en Facebook.
13. **`_unFriend.py`:** Eliminar a un usuario de la lista de amigos.

- **Estrategia Go:** Paquetes `pkg/features/thread` y `pkg/features/social` respectivamente, con clientes tipados para cada mutación y query GraphQL.
