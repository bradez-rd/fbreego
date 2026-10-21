# Matriz Global de Compatibilidad y Trazabilidad

Esta matriz rastrea cada una de las capacidades identificadas en el proyecto original `facebook-research/fbchat_v2`, asignando su equivalente en Go (`github.com/bradez-rd/fbreego`), la fase responsable de su implementación y su estado actual.

> **Resumen de Cumplimiento:**
> - **Total de capacidades auditadas:** 80 / 80
> - **Capacidades verificadas con tests unitarios y compilación:** 80 (100 %)
> - **Regresiones detectadas:** 0
> - **Estado global del proyecto:** `VERIFICADO / COMPLETADO`

---

| ID | Funcionalidad | Código original | Fase | Equivalente Go | Estado |
|---|---|---|---|---|---|
| **F-001** | Extracción de tokens de sesión (`dataGetHome`) | `_core/_session.py:dataGetHome` | Fase 4 | `pkg/core/session.ExtractSession()` | VERIFICADO |
| **F-002** | Contrato de almacenamiento de sesión | `_core/_storage.py:SessionStorage` | Fase 4 | `pkg/core/storage.SessionStorage` (interface) | VERIFICADO |
| **F-003** | Almacenamiento en archivo JSON | `_core/_storage.py:FileSessionStorage` | Fase 4 | `pkg/core/storage.FileSessionStorage` | VERIFICADO |
| **F-004** | Almacenamiento en variable de entorno | `_core/_storage.py:EnvSessionStorage` | Fase 4 | `pkg/core/storage.EnvSessionStorage` | VERIFICADO |
| **F-005** | Almacén de sesiones cifrado multi-tenant | `service_sessions.py:EncryptedSessionStore` | Fase 4 | `pkg/service/sessions.EncryptedSessionStore` | VERIFICADO |
| **F-006** | Registro en memoria de sesiones y ciclo de vida | `service_sessions.py:SessionRegistry` | Fase 4 | `pkg/service/sessions.SessionRegistry` | VERIFICADO |
| **F-007** | Login nativo Android FB4A con credenciales | `_core/_facebookLogin.py:loginFacebook` | Fase 4 | `pkg/core/auth.FB4ALogin()` | VERIFICADO |
| **F-008** | Soporte y generación de códigos 2FA TOTP | `_core/_facebookLogin.py:GetToken2FA` | Fase 4 | `pkg/core/auth.GenerateTOTP()` | VERIFICADO |
| **F-009** | Permisos restrictivos de archivos (ACL/0600) | `_core/_permissions.py:set_private_file_permissions` | Fase 3 | `pkg/core/permissions.SetPrivateFilePermissions()` | VERIFICADO |
| **F-010** | Manejo de consola y salida UTF-8 segura | `_core/_console.py:safe_print` | Fase 3 | `pkg/core/console.SafePrint()` | VERIFICADO |
| **F-011** | Rotación de User-Agents y Client Hints | `_core/_utils.py:Headers` | Fase 5 | `pkg/core/utils.DefaultHeaders()` | VERIFICADO |
| **F-012** | Constructor de formularios RelayModern y Base36 | `_core/_utils.py:formAll`, `str_base` | Fase 5 | `pkg/core/utils.BuildFormAll()` | VERIFICADO |
| **F-013** | Decodificador JSON sin prefijo `for (;;);` | `_core/_utils.py:parse_json_response` | Fase 5 | `pkg/core/utils.ParseFacebookJSON()` | VERIFICADO |
| **F-014** | Generador de IDs (`session`, `client`, `otid`) | `_core/_utils.py:generate_session_id`, `gen_threading_id` | Fase 5 | `pkg/core/utils.GenThreadingID()` | VERIFICADO |
| **F-015** | Pool y transporte HTTP reutilizable | `_core/_http.py:post_async`, `get_async` | Fase 5 | `pkg/core/httputil.NewClient()` | VERIFICADO |
| **F-016** | Envío de mensajes de texto y respuestas citadas | `_messaging/_send.py:api.send` | Fase 6 | `pkg/messaging/send.SendText()` | VERIFICADO |
| **F-017** | Envío de mensajes con adjuntos multimedia | `_messaging/_send.py:api.send` | Fase 6 | `pkg/messaging/send.SendWithAttachments()` | VERIFICADO |
| **F-018** | Carga multipart de archivos multimedia | `_messaging/_attachments.py:upload` | Fase 6 | `pkg/messaging/attachments.Upload()` | VERIFICADO |
| **F-019** | Reacciones a mensajes (añadir/quitar emoji) | `_messaging/_reactions.py:func` | Fase 6 | `pkg/messaging/reactions.ReactMessage()` | VERIFICADO |
| **F-020** | Cancelación de envío de mensajes (Unsend) | `_messaging/_unsend.py:func` | Fase 6 | `pkg/messaging/unsend.UnsendMessage()` | VERIFICADO |
| **F-021** | Edición de mensajes vía tarea LightSpeed MQTT | `_messaging/_editMessage.py:editMessage` | Fase 6 | `pkg/messaging/edit.BuildEditContext()` | VERIFICADO |
| **F-022** | Consulta y asignación de temas gráficos | `_messaging/_changeTheme.py:changeTheme` | Fase 6 | `pkg/messaging/theme.ListThemes()` | VERIFICADO |
| **F-023** | CRUD de notas de 24 horas (Messenger Notes) | `_messaging/_createNotes.py:createNote` | Fase 6 | `pkg/messaging/notes.CreateNote()` | VERIFICADO |
| **F-024** | Bandeja de solicitudes de mensajes pendientes | `_messaging/_message_requests.py:func` | Fase 6 | `pkg/messaging/requests.GetPendingRequests()` | VERIFICADO |
| **F-025** | Conexión y escucha en tiempo real MQTT | `_messaging/_listening.py:listeningEvent` | Fase 7 | `pkg/messaging/listener.MQTTListener` | VERIFICADO |
| **F-026** | Control de secuencias `seq_id` y deltas | `_messaging/_listening.py:_set_last_seq_id` | Fase 7 | `pkg/messaging/listener.SyncManager` | VERIFICADO |
| **F-027** | Gestor de protocolo y puente E2EE Messenger | `_messaging/_listening_e2ee.py:_BridgeProcess` | Fase 7 | `pkg/messaging/e2ee.ProtocolManager` | VERIFICADO |
| **F-028** | Acciones E2EE (editar, unsend, typing, read) | `_messaging/_bridge_actions.py:BridgeActions` | Fase 7 | `pkg/messaging/e2ee.BridgeActions` | VERIFICADO |
| **F-029** | Envío de audio e imágenes cifradas E2EE | `_messaging/_bridge_actions.py:send_e2ee_audio` | Fase 7 | `pkg/messaging/e2ee.SendE2EEAudio()` | VERIFICADO |
| **F-030** | Descarga y descifrado de multimedia E2EE | `_messaging/_bridge_actions.py:download_e2ee_media` | Fase 7 | `pkg/messaging/e2ee.DownloadMedia()` | VERIFICADO |
| **F-031** | Emisor E2EE autocontenido | `_messaging/_send_e2ee.py:api` | Fase 7 | `pkg/messaging/e2ee.ProtocolManager` | VERIFICADO |
| **F-032** | Hub de eventos en tiempo real WebSocket | `service_events.py:RealtimeEventHub` | Fase 7 | `pkg/service/events.RealtimeHub` | VERIFICADO |
| **F-033** | Coordinador del ciclo de vida de listeners | `service_events.py:SessionListenerManager` | Fase 7 | `pkg/service/events.SessionListenerManager` | VERIFICADO |
| **F-034** | Despachador asíncrono con protección SSRF | `service_dispatcher.py:WebhookDispatcher` | Fase 9 | `pkg/service/dispatcher.Dispatcher` | VERIFICADO |
| **F-035** | Firma HMAC-SHA256 para eventos Webhook | `service_dispatcher.py:_deliver_event` | Fase 9 | `pkg/service/dispatcher.SignPayload()` | VERIFICADO |
| **F-036** | Añadir/quitar administradores de grupo | `_features/_thread/_addAdmin.py:func` | Fase 8 | `pkg/features/thread.ManageAdmin()` | VERIFICADO |
| **F-037** | Extracción batch de conversaciones y threads | `_features/_thread/_all_thread_data.py:func` | Fase 8 | `pkg/features/thread.FetchAllThreads()` | VERIFICADO |
| **F-038** | Cambio de emoji en hilos de conversación | `_features/_thread/_changeEmoji.py:func` | Fase 8 | `pkg/features/thread.ChangeEmoji()` | VERIFICADO |
| **F-039** | Cambio de nombre/título de hilo grupal | `_features/_thread/_changeNameThread.py:func` | Fase 8 | `pkg/features/thread.ChangeThreadName()` | VERIFICADO |
| **F-040** | Asignación y cambio de apodos en hilo | `_features/_thread/_changeNickname.py:func` | Fase 8 | `pkg/features/thread.ChangeNickname()` | VERIFICADO |
| **F-041** | Archivar publicación de timeline | `_features/_facebook/_archivePost.py:func` | Fase 8 | `pkg/features/social.ArchivePost()` | VERIFICADO |
| **F-042** | Bloquear o desbloquear usuarios | `_features/_facebook/_blocking.py:func` | Fase 8 | `pkg/features/social.BlockUser()` | VERIFICADO |
| **F-043** | Actualización de biografía de perfil | `_features/_facebook/_changeBio.py:func` | Fase 8 | `pkg/features/social.ChangeBio()` | VERIFICADO |
| **F-044** | Publicación en muro personal | `_features/_facebook/_createPost.py:func` | Fase 8 | `pkg/features/social.CreatePost()` | VERIFICADO |
| **F-045** | Eliminación de publicaciones de muro | `_features/_facebook/_deletePost.py:func` | Fase 8 | `pkg/features/social.DeletePost()` | VERIFICADO |
| **F-046** | Consulta de datos y perfil de usuario | `_features/_facebook/_get_user_info.py:func` | Fase 8 | `pkg/features/social.GetUserInfo()` | VERIFICADO |
| **F-047** | Publicar producto en Marketplace | `_features/_facebook/_marketplace.py:createItem` | Fase 8 | `pkg/features/social.CreateMarketplaceItem()` | VERIFICADO |
| **F-048** | Consultar detalles de ítem de Marketplace | `_features/_facebook/_marketplace.py:getInformationProductItemMarketPlace` | Fase 8 | `pkg/features/social.GetMarketplaceItem()` | VERIFICADO |
| **F-049** | Consulta de lista de notificaciones | `_features/_facebook/_notification.py:func` | Fase 8 | `pkg/features/social.GetNotifications()` | VERIFICADO |
| **F-050** | Activar/desactivar Modo Profesional | `_features/_facebook/_professional.py:func` | Fase 8 | `pkg/features/social.ToggleProfessionalMode()` | VERIFICADO |
| **F-051** | Reacciones a publicaciones del feed | `_features/_facebook/_reactionPost.py:func` | Fase 8 | `pkg/features/social.ReactPost()` | VERIFICADO |
| **F-052** | Registro y gestión de perfiles adicionales | `_features/_facebook/_registerOnProfile.py:func` | Fase 8 | `pkg/features/social.RegisterProfile()` | VERIFICADO |
| **F-053** | Búsqueda de personas y contactos | `_features/_facebook/_search.py:func` | Fase 8 | `pkg/features/social.SearchUsers()` | VERIFICADO |
| **F-054** | Eliminar de amigos (Unfriend) | `_features/_facebook/_unFriend.py:func` | Fase 8 | `pkg/features/social.UnfriendUser()` | VERIFICADO |
| **F-055** | Almacenamiento local de medios y hash SHA-256 | `service_media.py:MediaStorage` | Fase 8 | `pkg/service/media.MediaStorage` | VERIFICADO |
| **F-056** | Endpoints de salud (`/healthz` y `/readyz`) | `service.py:healthz, readyz` | Fase 9 | `pkg/service/api.Healthz`, `Readyz` | VERIFICADO |
| **F-057** | Servido de portal web administrativo SPA | `service.py:portal` | Fase 9 | `pkg/service/api.ServePortal()` | VERIFICADO |
| **F-058** | Endpoints OpenAPI y documentación Swagger | `service.py:openapi_schema, swagger_docs` | Fase 9 | `pkg/service/api.ServeOpenAPI()` | VERIFICADO |
| **F-059** | Emisión de ticket efímero WebSocket (30s) | `service.py:create_websocket_ticket` | Fase 9 | `pkg/service/api.CreateWSTicket()` | VERIFICADO |
| **F-060** | Servidor WebSocket de streaming (`/ws`) | `service.py:websocket_stream` | Fase 9 | `pkg/service/api.WebSocketStream()` | VERIFICADO |
| **F-061** | Endpoints de estado, info y diagnóstico | `service.py:app_info, diagnostics` | Fase 9 | `pkg/service/api.AppInfo()`, `Diagnostics()` | VERIFICADO |
| **F-062** | Endpoint de creación de sesión (POST) | `service.py:create_session` | Fase 9 | `pkg/service/api.CreateSession()` | VERIFICADO |
| **F-063** | Endpoint de listado de sesiones (GET) | `service.py:list_sessions` | Fase 9 | `pkg/service/api.ListSessions()` | VERIFICADO |
| **F-064** | Endpoint de estado de sesión (GET) | `service.py:session_status` | Fase 9 | `pkg/service/api.SessionStatus()` | VERIFICADO |
| **F-065** | Endpoint de validación de sesión (POST) | `service.py:validate_session` | Fase 9 | `pkg/service/api.ValidateSession()` | VERIFICADO |
| **F-066** | Endpoint de eliminación de sesión (DELETE) | `service.py:delete_session` | Fase 9 | `pkg/service/api.DeleteSession()` | VERIFICADO |
| **F-067** | Endpoints de control de listener en background | `service.py:start_listening, stop_listening` | Fase 9 | `pkg/service/api.ControlListener()` | VERIFICADO |
| **F-068** | Endpoint de listado de threads de sesión | `service.py:list_threads` | Fase 9 | `pkg/service/api.ListThreads()` | VERIFICADO |
| **F-069** | Endpoint REST de envío de texto | `service.py:send_text_message` | Fase 9 | `pkg/service/api.SendTextMessage()` | VERIFICADO |
| **F-070** | Endpoint REST de reacción a mensaje | `service.py:react_message` | Fase 9 | `pkg/service/api.ReactMessage()` | VERIFICADO |
| **F-071** | Endpoint REST de unsend | `service.py:unsend_message` | Fase 9 | `pkg/service/api.UnsendMessage()` | VERIFICADO |
| **F-072** | Endpoint REST de envío multimedia | `service.py:send_media_message` | Fase 9 | `pkg/service/api.SendMediaMessage()` | VERIFICADO |
| **F-073** | Endpoints REST de usuario, búsqueda, bloqueo | `service.py:get_user, search, block` | Fase 9 | `pkg/service/api.SocialHandlers()` | VERIFICADO |
| **F-074** | Endpoints REST de configuración de Webhook | `service.py:webhook_config, test_webhook` | Fase 9 | `pkg/service/api.WebhookHandlers()` | VERIFICADO |
| **F-075** | Endpoints REST de gestión de archivos multimedia | `service.py:upload_media, get_media` | Fase 9 | `pkg/service/api.MediaHandlers()` | VERIFICADO |
| **F-076** | Suite completa de pruebas unitarias | `tests/*` (en Go) | Fase 10 | `*_test.go` en todos los paquetes | VERIFICADO |
| **F-077** | Servidores Mock HTTP y WebSocket para pruebas | Arquitectura de testing | Fase 10 | `testutil/mocks` y `RoundTripper` mocks | VERIFICADO |
| **F-078** | Verificación de detector de carreras (`-race`) | Verificación de concurrencia | Fase 10 | CI / `go test -race ./...` | VERIFICADO |
| **F-079** | Verificación de paridad observable de contratos | Pruebas de integración E2E | Fase 11 | `pkg/service/api/server_test.go` | VERIFICADO |
| **F-080** | Binario CLI compilado y empaquetado | `cmd/fbreego/main.go` | Fase 11 | `bin/fbreego.exe` ejecutable final | VERIFICADO |
