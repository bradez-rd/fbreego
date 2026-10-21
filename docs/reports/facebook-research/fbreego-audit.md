# Auditoría comparativa de `fbreego` frente a `facebook-research`

**Fecha de corte:** 2026-10-21  
**Repositorio actual:** `github.com/bradez-rd/fbreego`  
**Commit observado:** `8665f7bee6ec611e1694b7f774eefa376da854de`  
**Estado del informe:** análisis completo; 18 hallazgos confirmados y 2 hipótesis delimitadas.

## 1. Resumen ejecutivo

`fbreego` compila y su suite basada en mocks pasa, pero no tiene paridad operativa con las referencias en las dos capacidades centrales del producto: recepción en tiempo real y mensajería E2EE. El listener actual no abre ninguna conexión MQTT/DGW y el gestor E2EE devuelve un mapa sintético de éxito sin cifrar, transmitir ni descargar datos. La ruta de envío multimedia carga archivos a Facebook, pero después envía un mensaje de texto que no referencia los IDs de adjunto. Los webhooks tampoco reciben eventos porque no existe ninguna llamada productiva a `Dispatcher.Enqueue`.

Hay cuatro riesgos de seguridad críticos: autenticación REST que queda desactivada implícitamente, autenticación WebSocket inconsistente y aceptable con Basic vacío cuando no hay credenciales configuradas, cifrado de sesiones con una clave por defecto pública, y `tenantId` sin validar usado para construir rutas. El inspector de tráfico agrava la exposición al conservar cuerpos, respuestas y comandos cURL; su redacción no cubre el formato de cookies usado al crear sesiones.

La base sí aporta componentes aprovechables: separación por paquetes, almacenamiento AES-GCM, permisos POSIX, tickets WebSocket de un solo uso, HMAC para webhooks, control anti-SSRF inicial, deduplicación SHA-256 de medios, timeouts del servidor y tests unitarios de utilidades y adaptadores HTTP. Las dos dependencias directas declaradas están actualizadas según `go list -m -u -json all` en la fecha de corte. La estrategia recomendada es incremental: cerrar primero las falsas capacidades y los riesgos de seguridad, integrar un único transporte real para Messenger/E2EE, y luego ampliar API y Marketplace.

No se modificó código de producción ni se instalaron dependencias. El análisis incluyó los cambios locales ya presentes en el working tree; el informe no los atribuye al commit base.

## 2. Repositorio analizado

El descubrimiento de límites Git encontró un solo repositorio actual: la raíz `fbreego`. Los repositorios anidados bajo `facebook-research/` se trataron exclusivamente como referencias. No hay otros repositorios actuales pendientes.

| Atributo | Resultado |
|---|---|
| Lenguaje principal | Go |
| Módulo | `github.com/bradez-rd/fbreego` |
| Versión Go declarada | `1.27.2` |
| Tamaño observado | 101 archivos Go, aproximadamente 8.011 líneas |
| Tests | 30 archivos `*_test.go` |
| Dependencias directas | `github.com/coder/websocket v1.8.15`, `github.com/go-chi/chi/v5 v5.5.0` |
| Entrypoints | `main.go`, `cmd/fbreego/main.go` |
| Entrega/CI | Sin `Dockerfile`, workflows CI, linter o licencia en la raíz |
| Working tree preexistente | Modificaciones en `pkg/service/api/routes.go`, `tests/testutil/mock_facebook.go`, `web/portal.html`; archivos nuevos `pkg/service/api/handlers_tests.go`, `tests/integration/sender_receiver_e2e_test.go` |

## 3. Repositorios de referencia utilizados

| Referencia | Snapshot observado | Uso en la comparación | Cautelas |
|---|---|---|---|
| `facebook-research/fbchat_v2` | paquete `2.3.3` | Contrato directo del servicio, listeners, eventos, adjuntos, sesiones y features sociales | Es una API no oficial; también contiene deuda y no se asumió superior por defecto |
| `facebook-research/mautrix-meta` | `c4099692`, 2026-10-19 | Transporte DGW/LightSpeed, sincronización, retries, uTLS y E2EE en Go | AGPL-3.0; no copiar ni enlazar sin revisión legal/arquitectónica |
| `facebook-research/fb-messenger-e2ee` | `446b6af5`, 2026-09-24 | Noise, Signal, device store, prekeys, media cifrada y mapeo de eventos | AGPL-3.0 y dependencia nativa de libsignal |
| `facebook-research/fca-unofficial` | `a8789a9f`, 2026-08-18 | Inventario de acciones Messenger y refresco de `fb_dtsg` | MIT, pero usa dependencias/patrones Node antiguos; referencia funcional, no base arquitectónica |
| `facebook-research/facebook-marketplace-mcp` | `30fd6d15`, 2026-04-09 | Búsqueda, ubicación, monitores, rate limiting y captura de `doc_id` | Sin licencia visible en la raíz; los `doc_id` rotan |

La prioridad técnica se dio a `fbchat_v2` por ser el contrato migrado y a `mautrix-meta`/`fb-messenger-e2ee` para validar que una implementación real requiere bastante más que una fachada RPC. `fca-unofficial` solo se usó para descubrir operaciones y patrones de recuperación. `facebook-marketplace-mcp` se usó para oportunidades específicas de Marketplace.

## 4. Estado actual de las funcionalidades

| Área | Estado comprobado | Evaluación |
|---|---|---|
| Sesiones | Creación, validación HTTP, registro en memoria y persistencia AES-GCM | Parcial; clave insegura por defecto, aislamiento tenant incompleto y errores de persistencia ignorados |
| Mensajes de texto, reacción y unsend | Adaptadores HTTP implementados y cubiertos con mocks | Parcial; sin refresco de token, retries ni validación semántica uniforme |
| Adjuntos | Upload Mercury implementado | Parcial; la API de “media message” no asocia los adjuntos al mensaje |
| Recepción en tiempo real | Tipos, canal y gestor de estado presentes | No operativa; no existe conexión/subscripción MQTT/DGW |
| E2EE | Fachada de acciones y `ProtocolManager` presentes | No operativa; responde éxito sintético, sin Noise/Signal/device store |
| WebSocket | Hub, tickets y escritura asíncrona presentes | Parcial; no recibe mensajes reales y la política de autenticación/origen es insegura |
| Webhooks | Configuración cifrada, HMAC, cola y retry en memoria | No integrada; ningún flujo productivo encola eventos |
| Marketplace | Crear/detalle como funciones; solo detalle expuesto por REST | Parcial y frágil; sin búsqueda, ubicación, monitores, rate limit ni descubrimiento de operaciones |
| Social/thread | Varias funciones Go existen | Parcial; muchas no están integradas en la API y faltan operaciones de referencias recientes |
| Medios locales | Hash, deduplicación y endpoints CRUD | Parcial; límites incompletos y catálogo solo en memoria |
| OpenAPI/documentación | Endpoint y README presentes | Desactualizados respecto de las 41 rutas registradas y la configuración real |
| Tests | `go test ./...` y `go vet ./...` pasan | Útiles para contratos locales, insuficientes para demostrar paridad real |

## 5. Features faltantes

| ID | Categoría | Feature | Estado actual | Referencia | Evidencia | Recomendación | Prioridad | Esfuerzo | Riesgo |
|---|---|---|---|---|---|---|---|---|---|
| FBR-001 | Faltante | Recepción real de mensajes | El constructor `MQTTListener` no se usa y `listenLoop` solo espera con backoff. La ruta `/listen/start` ejecuta `RecordActivity` una vez y el worker termina. | `fbchat_v2/service.py:_run_session_listener`; `mautrix-meta/pkg/messagix/client.go:Connect`; `fb-messenger-e2ee/.../listen.action.ts` | `pkg/messaging/listener/mqtt_client.go:61-86`; `pkg/service/api/handlers_session.go:80-99`; `pkg/service/events/listener_manager.go:46-75` | Definir una interfaz `RealtimeTransport`, implementar conexión, subscribe, reconexión, cursor persistente y normalización de eventos; no anunciar `listening` hasta handshake/sync confirmados. | Crítica | Alto | Alto |
| FBR-002 | Faltante | Cifrado E2EE operativo | `ProtocolManager.Call` acepta nombres conocidos y devuelve `status: success` con el payload original. No hay Noise, Signal, prekeys, device store ni transporte. Readiness lo declara siempre listo. | `fb-messenger-e2ee/src/e2ee/application/e2ee-client.ts`; `.../signal/signal-manager.ts`; `.../store/device-store.ts`; `mautrix-meta/pkg/messagix/e2ee-client.go` | `pkg/messaging/e2ee/protocol.go:29-49`; `pkg/service/api/server.go:206-210`; `pkg/messaging/e2ee/e2ee_test.go:8-64` | Sustituir la fachada por un adaptador real. Preferir integración detrás de interfaz/proceso aislado y pruebas de vectores; revisar AGPL antes de reutilizar código. Hasta entonces devolver `NOT_IMPLEMENTED` y readiness no listo. | Crítica | Alto | Alto |
| FBR-003 | Faltante | Entrega de eventos a webhooks y eventos de mensaje | El dispatcher se inicia, pero hay cero llamadas a `Enqueue` en código Go. `EmitMessageReceived` no tiene call sites productivos. | `fbchat_v2/service_events.py:70-123`; `fbchat_v2/service.py:892-965,1129-1258` | `pkg/service/dispatcher/dispatcher.go:78-96`; búsqueda global sin `.Enqueue(`; `pkg/service/events/hub.go:104-112` | Crear un bus interno único: cada evento normalizado se publica una vez y se fan-out a WebSocket/webhook. Añadir métricas de descartes y pruebas de contrato. | Alta | Medio | Alto |
| FBR-004 | Faltante | API para capacidades ya implementadas | Cambios de tema/nombre/emoji/apodo/admin, notas, solicitudes, posts, notificaciones, modo profesional y creación Marketplace existen como paquetes, pero no tienen rutas REST. | Inventario de `fbchat_v2/_features` y `fca-unofficial/src/actions/index.js` | `pkg/features/**`; `pkg/messaging/{edit,notes,requests,theme}`; `pkg/service/api/routes.go:122-168` | Exponer solo operaciones validadas, por lotes pequeños, con DTOs, autorización por sesión y OpenAPI generado. No exponer mutaciones de alto riesgo hasta tener contract tests reales. | Media | Medio | Medio |
| FBR-005 | Faltante | Operaciones Messenger modernas | Faltan historia paginada, forward, polls, crear/eliminar/mutar grupos, add/remove member, mute, delivered/read/seen, typing y friend/message request workflows. | `fb-messenger-e2ee/src/actions/{messaging,thread,user}`; `fca-unofficial/src/actions/index.js` | Ausencia en `pkg/messaging`, `pkg/features` y `pkg/service/api/routes.go` | Priorizar `typing`, `mark-read`, historia y membresía por utilidad. Reutilizar el futuro transporte real en vez de crear endpoints HTTP independientes por operación. | Media | Alto | Medio |
| FBR-006 | Faltante | Marketplace de búsqueda y seguimiento | Solo hay create/detail; create ni siquiera está expuesto. No hay búsqueda por radio/precio/categoría, resolución de ubicación, cursor, monitor ni rate limit. | `facebook-marketplace-mcp/src/facebook/client.ts:211-259`; `src/tools/{search,location,monitor}.ts`; `src/utils/rate-limit.ts` | `pkg/features/social/marketplace.go:26-84`; `pkg/service/api/routes.go:146` | Incorporar primero búsqueda read-only + ubicación + rate limiter. Añadir monitores después de contar con persistencia y dispatcher fiables. | Media | Medio | Medio |

## 6. Features desactualizadas

| ID | Categoría | Feature | Estado actual | Referencia | Evidencia | Recomendación | Prioridad | Esfuerzo | Riesgo |
|---|---|---|---|---|---|---|---|---|---|
| FBR-007 | Desactualizada | Autenticación y configuración pública | El README promete JWT, pero el código usa Basic Auth. Sin env vars, REST queda abierto; WebSocket considera auth requerida y acepta Basic con usuario/clave vacíos. `app/info` usa una tercera regla distinta. | `fbchat_v2/service.py:319-349,403-520` mantiene una política común de auth/tickets | `README.md:117-132`; `pkg/service/api/middleware.go:43-70`; `pkg/service/api/routes.go:60-70,174-194` | Centralizar una política fail-closed, exigir secreto/credenciales al iniciar producción, eliminar password en query, ligar tickets a sesión/identidad y documentar Basic u OIDC/JWT real. | Crítica | Medio | Alto |
| FBR-008 | Desactualizada | Envío multimedia | Los archivos se suben, pero `uploadedIDs` solo se devuelve en JSON; `SendText` fuerza `has_attachment=false` y nunca recibe los IDs. | `fbchat_v2/_messaging/_send.py:_build_form`; `fbchat_v2/service.py:1217-1260` | `pkg/service/api/handlers_messages.go:115-155`; `pkg/messaging/send/sender.go:19-83` | Añadir un modelo de attachment tipado y una única operación `SendMessage` que construya `image_ids`/`video_ids`/`audio_ids`/`file_ids`; fallar si falta un medio, sin silencios. | Crítica | Medio | Alto |
| FBR-009 | Desactualizada | Transporte HTTP y resiliencia | Existe `httputil.NewClient`, pero el runner no lo usa. Las operaciones salvo extracción de sesión no comprueban `StatusCode`, no refrescan `fb_dtsg`, no aplican retry por 429/5xx ni rate limit. TLS sigue siendo el fingerprint estándar de Go. | `mautrix-meta/pkg/messagix/httpclient/http.go:142-179,485-517`; `facebook-marketplace-mcp/src/facebook/client.ts:53,186-190`; `fca-unofficial/.../refreshFb_dtsg.js` | `pkg/service/runner/runner.go:70-89`; `pkg/service/sessions/registry.go:24-31`; `pkg/core/httputil/client.go`; búsquedas de `resp.StatusCode` | Inyectar un único cliente productivo con clasificación de errores, retry con jitter/`Retry-After`, refresco de token de una sola vez y límites por sesión. Evaluar uTLS solo si la evidencia operativa lo justifica. | Alta | Alto | Alto |
| FBR-010 | Desactualizada | Operaciones GraphQL privadas | Los `doc_id` están hard-codeados y divergen de ambas referencias locales; no existe mecanismo de captura/descubrimiento ni contract test live. | `fbchat_v2/_features/**`; `facebook-marketplace-mcp/src/facebook/queries.ts:1-13` y script `capture-queries` | `pkg/features/social/*.go`; `pkg/messaging/{notes,reactions,requests,theme}` | Registrar operaciones en un catálogo versionado con fecha/fuente, detectar errores de operación y habilitar actualización controlada. Ver HYP-001 antes de declarar funciones rotas. | Alta | Medio | Alto |
| FBR-011 | Desactualizada | README y OpenAPI | README usa env vars no leídas, JWT inexistente y rutas ausentes; termina incompleto. El esquema manual documenta 5 paths frente a 41 rutas registradas y no describe seguridad/DTOs/respuestas. | FastAPI genera el esquema desde rutas/modelos en `fbchat_v2/service.py` | `README.md:79-211`; `pkg/service/api/openapi.go:9-58`; `pkg/service/api/routes.go:17-171` | Generar OpenAPI desde una fuente de verdad o añadir tests que comparen router/esquema. Corregir configuración real: `PORT`, `FBCHAT_SESSIONS_DIR`, `FBCHAT_SESSION_SECRET`, `FBCHAT_API_USERNAME/PASSWORD`. | Alta | Medio | Medio |
| FBR-012 | Desactualizada | Documentación de migración y estado “80/80” | La matriz declara MQTT, E2EE, adjuntos, `-race`, goleak y paridad como verificados aunque varias implementaciones son stubs o no existen. Cita archivos que no están en esas rutas. | Evidencia ejecutable de las referencias y suite real | `facebook-research/fbchat_v2/docs/migracion-go/MATRIZ_COMPATIBILIDAD.md:F-017,F-025,F-027,F-034,F-078,F-079`; archivos actuales | Cambiar estados a `implementado`, `mock-verificado`, `live-verificado`, `stub` o `no implementado`; exigir evidencia por nivel y fecha. | Alta | Bajo | Medio |

## 7. Features mejorables

| ID | Categoría | Feature | Estado actual | Referencia | Evidencia | Recomendación | Prioridad | Esfuerzo | Riesgo |
|---|---|---|---|---|---|---|---|---|---|
| FBR-013 | Mejorable | Cifrado y permisos de secretos | Sesiones y webhook usan AES-GCM, pero derivan la clave con SHA-256 de un secreto por defecto público. Errores de mkdir/permisos/save/load se ignoran; Windows oculta errores de `icacls`. | `fbchat_v2/service_sessions.py:_fernet`; prácticas de device store de E2EE | `pkg/service/sessions/encrypted_store.go:26-47,96-127`; `pkg/service/webhook/config_store.go:58-76`; `pkg/core/permissions/permissions_windows.go:13-31` | Hacer obligatorio un secreto de alta entropía o keystore, usar KDF con salt/versionado, propagar fallos, soportar rotación y fallar startup si no se puede proteger el directorio. | Crítica | Medio | Alto |
| FBR-014 | Mejorable | Aislamiento multi-tenant y rutas | `tenantId` controlado por el cliente entra en `filepath.Join`; `..` puede escapar de `stateDir`. El lookup de sesión no verifica tenant y la auth es global. | Las referencias conservan tenant en eventos/registro, pero no son un modelo completo de autorización | `pkg/core/types/service_dto.go:9-14`; `pkg/service/sessions/encrypted_store.go:85-93`; `pkg/service/sessions/registry.go:146-156` | Validar tenant con allowlist/slug, comprobar ruta final bajo root, asociar principal→tenant y aplicar el filtro en cada `Get/Validate/Delete`. | Crítica | Medio | Alto |
| FBR-015 | Mejorable | Inspector de tráfico y privacidad | Lee cuerpos completos antes de truncar, guarda request/response/cURL en memoria y los expone por API. La regex no redacciona `{name:"xs",value:"..."}` ni campos arbitrarios sensibles. | `mautrix-meta` usa redacción especializada de logs; las referencias no exponen un replay completo por defecto | `pkg/service/traffic/{middleware,transport,model}.go`; `pkg/core/logger/logger.go:9-25`; `pkg/service/api/routes.go:162-165` | Deshabilitar por defecto, aplicar streaming con límites antes de leer, allowlist de campos, hash/omit de cuerpos sensibles, RBAC separado y TTL. Nunca generar cURL reproducible con credenciales. | Crítica | Medio | Alto |
| FBR-016 | Mejorable | Protección SSRF de webhooks | Se valida DNS antes de `http.Client.Do`, pero el cliente vuelve a resolver y sigue redirects sin revalidar; existe ventana de DNS rebinding y redirect a red privada. `time.Sleep` ignora cancelación durante backoff. | La referencia Python también debe tratarse con cautela; no se considera superior aquí | `pkg/service/dispatcher/ssrf.go:35-69`; `pkg/service/dispatcher/dispatcher.go:39-47,115-170` | Resolver y fijar IP en `DialContext`, validar cada redirect y destino, restringir esquemas/puertos, usar timer seleccionable por contexto y limitar respuesta. | Alta | Medio | Alto |
| FBR-017 | Mejorable | Medios y consumo de recursos | Multipart declara 50 MB, pero JSON base64 y `MediaStorage.Store` no limitan; el transporte lee uploads/respuestas completos. El catálogo de medios no persiste, por lo que reinicios dejan archivos huérfanos. MIME es confiado. | `fbchat_v2/service_media.py` valida y mantiene metadata; `fb-messenger-e2ee` separa crypto/upload | `pkg/service/api/handlers_media.go:15-62`; `pkg/service/media/storage.go:42-119`; `pkg/service/traffic/transport.go:39-99` | `MaxBytesReader` + `LimitReader`, cuotas por tenant, detección MIME, catálogo persistente/reconciliación y limpieza por TTL. Excluir binarios del inspector. | Alta | Medio | Medio |
| FBR-018 | Mejorable | Calidad de tests y operación | `go test`/`vet` pasan, pero API 17,5%, dispatcher 14,7%, events 22,5%, listener 25% y sessions 31,4%. “E2E” reescribe tráfico a un mock; el endpoint productivo de tests puede enviar/reaccionar/unsend y fabrica IDs `mid.mock_*`. `-race` no pudo ejecutarse: CGO=0 y no hay compilador C. No hay CI/goleak. | `fb-messenger-e2ee/tests/unit` cubre codecs, Noise, Signal y media; referencias mantienen integración separada | `tests/integration/*.go`; `pkg/service/api/handlers_tests.go:182-346`; ausencia de `.github`; salida de verificación de esta auditoría | Retirar o feature-flaggear `/api/v1/tests/e2e`; separar contract/live tests con credenciales explícitas; probar fallos, límites, auth y persistencia. Añadir CI Linux con race, cobertura y fuzzing de parsers. | Alta | Medio | Alto |

## 8. Comparación técnica con las referencias

| Dimensión | `fbreego` | Referencias | Conclusión |
|---|---|---|---|
| Tiempo real | Loop de espera y worker efímero | `fbchat_v2` crea listener E2EE, espera conexión, consume cola y emite eventos; `mautrix-meta` conecta DGW/LightSpeed | Falta el núcleo de recepción |
| E2EE | Fachada RPC sintética | Signal/Noise, prekeys, device identity, sesiones, fanout, retry y media crypto | No existe equivalencia funcional |
| Mensajería clásica | Forms HTTP sencillos | Forms + validación, adjuntos tipados, retries y tareas socket | Base útil, pero frágil y sin recuperación |
| Eventos | Hub WebSocket aislado; dispatcher sin productores | Bus central que distribuye a WebSocket y webhook | Integración incremental de bajo acoplamiento |
| Marketplace | Create/detail hard-coded | Search/location/cursor/monitor/rate limit y captura de queries | Buena oportunidad read-only; no copiar IDs ciegamente |
| HTTP/TLS | `net/http` estándar; helper optimizado no cableado | Retry consciente de status/contexto; uTLS en `mautrix-meta` | Centralizar cliente antes de ampliar features |
| Estado | Sesión cifrada en archivos; medios/colas/auditoría en memoria | Persistencia de device keys/cursors y, en `mautrix-meta`, base de datos | Persistir solo estado necesario, con esquema/versionado |
| Validación | DTOs simples sin validadores | Pydantic/Zod/modelos tipados | Añadir validación en frontera y errores estables |
| Pruebas | Mocks HTTP y flujos locales | Vectores de criptografía/codecs y scripts live separados | Cambiar criterio de “paridad” y cubrir protocolos reales |
| Licencias | Sin licencia raíz | Apache-2.0 (`fbchat_v2`), MIT (`fca`), AGPL-3.0 (`mautrix-meta`/`fb-messenger-e2ee`) | Diseñar integración con revisión legal; no copiar AGPL automáticamente |

Las referencias no deben fusionarse entre sí ni incorporarse completas. `fca-unofficial` expone un inventario amplio, pero sus dependencias `request`, `bluebird` y MQTT v3 son una advertencia de antigüedad. `mautrix-meta` es técnicamente más completo, pero mucho más pesado y AGPL. `fb-messenger-e2ee` demuestra los componentes mínimos de E2EE, no una dependencia automáticamente compatible. La arquitectura aconsejable es una interfaz estrecha de transporte/protocolo con implementaciones intercambiables.

## 9. Recomendaciones de implementación

1. **Cerrar falsos positivos y superficie crítica.** Marcar listener/E2EE no disponibles, retirar el endpoint de tests de producción, hacer auth fail-closed y bloquear startup sin secreto seguro.
2. **Corregir integridad funcional.** Unificar envío de texto/adjuntos, cablear eventos al bus y propagar errores de persistencia. Estas mejoras no requieren todavía E2EE.
3. **Crear contratos internos.** Introducir `MessengerTransport`, `EventBus`, `SessionStore` y errores tipados. El servicio HTTP no debe conocer GraphQL/MQTT/Signal directamente.
4. **Implementar recepción real.** Elegir DGW/LightSpeed/MQTT según pruebas actuales; persistir cursor/seq y probar reconexión/duplicados. Normalizar un único `MessageReceived`.
5. **Integrar E2EE detrás del contrato.** Evaluar implementación limpia, subproceso o dependencia con revisión de licencia. Exigir vectors/tests de Noise, Signal, device state y media antes de habilitar readiness.
6. **Endurecer HTTP.** Cliente único inyectado, límites, retries cancelables, token refresh, rate limit y observabilidad sin secretos.
7. **Ampliar features de forma incremental.** Primero lectura/historia/typing/mark-read y Marketplace search; después mutaciones sociales y monitores.
8. **Restablecer contrato público.** OpenAPI generado o verificado contra router, README ejecutable, ejemplos reales de Basic/JWT elegido y matriz de capacidades con niveles de evidencia.

## 10. Riesgos y compatibilidad

| Riesgo | Compatibilidad/impacto | Mitigación |
|---|---|---|
| Protocolos privados de Meta | Los `doc_id`, payloads y endpoints pueden rotar sin aviso | Catálogo versionado, detección de error, smoke tests opt-in y rollback |
| Bloqueos/checkpoints de cuenta | Automatización, retries agresivos o fingerprint inconsistente pueden activar antifraude | Rate limits conservadores, jitter, circuit breaker y cuentas de prueba |
| AGPL | Vincular/copiar `mautrix-meta` o `fb-messenger-e2ee` puede imponer obligaciones al servicio | Revisión legal; proceso separado no elimina automáticamente obligaciones |
| Migración de sesiones | Cambiar KDF/formato puede volver ilegibles archivos existentes | Encabezado de versión, migrador, backup y rotación dual temporal |
| Cambio de auth | Fail-closed rompe despliegues hoy implícitamente abiertos | Modo de compatibilidad con warning y fecha de corte; health/readiness explícitos |
| Persistencia multi-tenant | Corregir paths/ownership puede mover archivos | Migración validada y auditoría de rutas antes de activar aislamiento |
| E2EE/device state | Perder identity/prekeys rompe sesiones o fuerza re-registro | Escrituras atómicas, backups cifrados y pruebas de recuperación |
| Webhook durable | Pasar de cola en memoria a persistente cambia orden/duplicados | IDs de evento, idempotencia, at-least-once documentado y DLQ |

### Hipótesis que requieren validación

| ID | Hipótesis | Evidencia disponible | Validación necesaria |
|---|---|---|---|
| HYP-001 | Parte de las operaciones GraphQL hard-codeadas ya no funciona contra Meta | Los IDs Go difieren de `fbchat_v2` y Marketplace; las referencias advierten rotación | Cuenta de prueba, captura controlada de operaciones y comparación de respuestas; no usar cuenta personal |
| HYP-002 | El fingerprint TLS estándar de Go aumenta bloqueos/checkpoints | `httputil` solo configura TLS/HTTP2; `mautrix-meta` usa uTLS Chrome explícito | A/B controlado con métricas de errores/checkpoints; no introducir uTLS sin evidencia |

## 11. Roadmap priorizado

| Fase | Objetivo | Hallazgos | Resultado verificable |
|---|---|---|---|
| P0 — 0 a 2 semanas | Seguridad y verdad operacional | FBR-002, 007, 013, 014, 015, 018 | Auth fail-closed; secreto obligatorio; path traversal bloqueado; test endpoint deshabilitado; readiness no promete E2EE |
| P0 — 1 a 3 semanas | Corregir funciones rotas | FBR-003, 008 | Adjuntos aparecen en el mensaje real; evento de prueba llega a WS y webhook exactamente una vez |
| P1 — 3 a 8 semanas | Transporte real y resiliencia | FBR-001, 009, 010, 016 | Recepción/reconexión con cursor persistido; retries cancelables; refresh de token; SSRF resistente a redirect/rebinding |
| P1 — 6 a 12 semanas | E2EE real | FBR-002 | Vectores crypto, registro/device store, envío y recepción E2EE live con cuenta de prueba |
| P2 — 2 a 6 semanas | Contrato y calidad | FBR-011, 012, 017, 018 | Router/OpenAPI sincronizados; CI race/coverage; docs y matriz con nivel de evidencia |
| P2 — incremental | Ampliación funcional | FBR-004, 005, 006 | Features read-only y de mensajería priorizadas con contract tests y feature flags |

Dependencias: FBR-001 debe preceder eventos recibidos y buena parte de FBR-002; FBR-009 precede ampliaciones GraphQL; FBR-003 precede monitores Marketplace; FBR-013/014 deben cerrarse antes de migrar o añadir más sesiones.

## 12. Evidencias y rutas de archivos

### Repositorio actual

- `go.mod:1-8`: módulo, Go 1.27.2 y dos dependencias directas.
- `pkg/service/api/routes.go:17-171`: 41 rutas registradas y middleware.
- `pkg/service/api/routes.go:174-204`: autenticación WebSocket y origen `*`.
- `pkg/service/api/middleware.go:43-87`: Basic Auth fail-open y CORS wildcard con credentials.
- `pkg/service/api/server.go:180-222`: readiness y E2EE siempre “ready”.
- `pkg/service/api/handlers_session.go:80-99`: worker de escucha efímero.
- `pkg/messaging/listener/mqtt_client.go:61-86`: loop simulado, sin broker.
- `pkg/messaging/e2ee/protocol.go:29-49`: éxito sintético por nombre de método.
- `pkg/service/api/handlers_messages.go:115-155`: carga IDs de adjuntos y luego llama a `SendText`.
- `pkg/messaging/send/sender.go:19-83`: form de texto con `has_attachment=false`.
- `pkg/service/dispatcher/dispatcher.go:78-176`: cola, retry y sleep; no hay productores en el repositorio.
- `pkg/service/events/hub.go:66-112`: hub y emisores sin entrada de mensajes real.
- `pkg/service/sessions/encrypted_store.go:26-47,85-127`: secreto por defecto, tenant path y errores ignorados.
- `pkg/service/sessions/registry.go:146-193`: puntero mutable devuelto fuera del lock; mutaciones concurrentes posibles.
- `pkg/core/permissions/permissions_windows.go:13-31`: errores de ACL convertidos en éxito.
- `pkg/service/traffic/middleware.go:32-99` y `transport.go:33-113`: captura completa de cuerpos.
- `pkg/core/logger/logger.go:9-25`: redacción basada en regex limitada.
- `pkg/service/dispatcher/ssrf.go:35-79`: validación DNS separada del dial.
- `pkg/service/media/storage.go:42-119`: catálogo en memoria y copia sin límite interno.
- `pkg/service/api/openapi.go:9-58`: solo 5 paths en el esquema manual.
- `README.md:79-211`: requisitos/configuración/rutas divergentes y documento incompleto.
- `pkg/service/api/handlers_tests.go:182-346`: endpoint de test destructivo, warnings como éxito e IDs mock.
- `tests/integration/service_e2e_test.go:25-84`: transporte redirigido a mock.
- `facebook-research/fbchat_v2/docs/migracion-go/MATRIZ_COMPATIBILIDAD.md`: afirmaciones 80/80 que no corresponden al código operativo.

### Referencias

- `facebook-research/fbchat_v2/service.py:773-995`: listener real, cola, conexión, normalización y emisión.
- `facebook-research/fbchat_v2/service_events.py:70-123`: fan-out WebSocket + webhook.
- `facebook-research/fbchat_v2/_messaging/_send.py:50-156`: validación y campos de adjuntos.
- `facebook-research/fb-messenger-e2ee/src/e2ee/store/device-store.ts:1-80`: stores Signal y Noise.
- `facebook-research/fb-messenger-e2ee/src/e2ee/signal/signal-manager.ts`: cifrado/descifrado Signal.
- `facebook-research/fb-messenger-e2ee/src/e2ee/transport/noise/noise-socket.ts`: conexión Noise real.
- `facebook-research/mautrix-meta/pkg/messagix/client.go:246-331`: ciclo de conexión.
- `facebook-research/mautrix-meta/pkg/messagix/syncManager.go`: cursores y sincronización por bases LightSpeed.
- `facebook-research/mautrix-meta/pkg/messagix/httpclient/http.go:142-179,485-517`: uTLS y retry contextual.
- `facebook-research/facebook-marketplace-mcp/src/facebook/client.ts:43-259`: rate limit, tokens, errores y operaciones Marketplace.
- `facebook-research/facebook-marketplace-mcp/src/facebook/queries.ts:1-13`: rotación y captura de `doc_id`.
- `facebook-research/fca-unofficial/src/actions/index.js:1-65`: inventario de acciones adicionales.

### Comandos de verificación ejecutados

| Comando | Resultado |
|---|---|
| `go version` | `go1.27.2 windows/amd64` |
| `go list ./...` | 36 paquetes listados correctamente |
| `go test ./...` | Pasa |
| `go vet ./...` | Pasa, sin salida |
| `go test -cover ./...` | Pasa; coberturas por paquete entre 0% y 85,7%; API 17,5% |
| `go test -race ./...` | No ejecutado: `-race requires cgo`; `CGO_ENABLED=0` y `gcc` ausente |
| `go list -m -u -json all` | Sin actualización reportada para las dos dependencias directas |

## Cierre del ciclo

- Análisis de `fbreego`: **completo**.
- Informe individual: **creado** en `reports/facebook-research/fbreego-audit.md`.
- Repositorios actuales adicionales pendientes: **ninguno**.
- Código de producción/dependencias modificado durante la auditoría inicial: **no**.

## Estado de remediación posterior

**Fecha:** 2026-10-21. Esta sección registra la implementación solicitada después de cerrar la auditoría; no altera la evidencia histórica del diagnóstico.

| Hallazgos | Estado | Implementación comprobable |
|---|---|---|
| FBR-007 | Mitigado | Auth REST/WS unificada y fail-closed por defecto; validación de startup; CORS/WS por allowlist; tickets de un uso ligados a sesión; eliminado el password en query. |
| FBR-008 | Implementado, mock-verificado | `SendWithAttachments` enlaza IDs tipados y el handler deja de silenciar fallos de staging/upload. |
| FBR-003 | Parcial | `RealtimeHub.Publish` distribuye a WebSocket y `Dispatcher.Enqueue`; operaciones locales publican eventos. La recepción externa depende aún de FBR-001. |
| FBR-002 / FBR-001 | Veracidad operacional mitigada | E2EE devuelve `ErrProtocolUnavailable`; listener devuelve `501`/error tipado y readiness indica no disponible. El protocolo/transporte real sigue pendiente. |
| FBR-013 | Parcial | Secreto de 32+ caracteres obligatorio, startup no-cero ante configuración insegura, errores de actividad/permisos/persistencia propagados y JSON corrupto protegido contra sobrescritura. Faltan KDF versionada y rotación. |
| FBR-014 | Parcial | `tenantId` usa allowlist y comprobación de confinamiento de ruta. Sigue pendiente autorización principal→tenant. |
| FBR-015 | Parcial | Inspector opt-in, previews streaming de 64 KiB y redacción ampliada. Siguen pendientes RBAC dedicado y TTL. |
| FBR-016 | Implementado | `DialContext` valida la IP exacta, redirects revalidados y backoff cancelable. |
| FBR-017 | Parcial | Uploads y almacenamiento tienen límites previos; el directorio se valida en startup/readiness y los nombres se endurecieron. Siguen pendientes catálogo persistente, cuotas y validación MIME fuerte. |
| FBR-011 / FBR-018 | Parcial | README corregido, OpenAPI cubre rutas estables con test de sincronización, endpoint E2E opt-in, pruebas débiles reparadas y CI Linux configurado con vet, race y cobertura. Sigue pendiente ejecutar contratos live aislados. |

Durante la remediación se detectaron cookies reales versionadas en `tests/default_cookies/ss1.json` y `ss2.json`. Ambos archivos quedaron ignorados y retirados del índice sin borrar las copias locales. Como ya aparecieron en el commit remoto `81f42e4`, es obligatorio revocar/rotar esas sesiones y posteriormente purgar el historial remoto mediante un procedimiento coordinado; borrar solo el commit actual no invalida los secretos expuestos.

Verificación posterior: `go test ./...`, `go vet ./...` y `git diff --check` pasan sin instalar dependencias nuevas.
