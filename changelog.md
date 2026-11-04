# Changelog — fbreego

Todas las modificaciones notables a este proyecto son documentadas en este archivo.
El formato se basa en [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/).

---

## [2.3.4-dev] - 2026-11-04

### Añadido
- Búsqueda en Facebook Marketplace (`SearchMarketplace`) con GraphQL `doc_id: 7111939778879383`, soporte de coordenadas, radio, filtros de precio y paginación por cursor (referencia `docs/research/facebook-marketplace-mcp`).
- Actualización de la mutación de publicación en Marketplace (`CreateMarketplaceItem`) con `useCometMarketplaceListingCreateMutation` y `doc_id: 5033081016747999`, taxonomía de categorías de Meta y soporte de imágenes asociadas `photo_ids` (referencia `docs/research/fbchat_v2`).
- Nuevos endpoints REST `GET /api/v1/sessions/{sessionId}/marketplace/search` y `POST /api/v1/sessions/{sessionId}/marketplace` con validación DTO y documentación OpenAPI 3.0.3.
- Extracción dinámica de metadatos de sesión y broker (`Region`, `MQTTEndpoint`, `MQTTClientID`, `MQTTAppID`) en `pkg/core/session/session.go`.
- Mecanismo de refresco de tokens (`RefreshFbDtsg` y `RefreshSessionTokens`) para renovación automática de `fb_dtsg` y `jazoest`.
- Soporte para consulta inicial de número de secuencia (`sync_sequence_id`) previa al handshake MQTT mediante GraphQL batch.
- Suscripción completa a tópicos MQTT de Messenger (`/t_ms`, `/ls_req`, `/ls_resp`, typing, presence) y normalización del payload CONNECT (`chat_on: true` booleano).
- Inicialización canónica y expansión del directorio `docs/research/` con 13 submódulos y fuentes de referencia:
  - Go: `mautrix-meta`, `motofb`, `hermes`
  - Python: `fbchat_v2`, `fbchat`, `unzuckify`
  - JavaScript / TypeScript: `fca-unofficial`, `fca-unofficial-tormairedusi`, `fca-unofficial-dongp06`, `facebook-chat-api`, `fb-messenger-e2ee`, `facebook-marketplace-mcp`
  - C / Bajo nivel: `purple-facebook`
- Archivo de configuración `.gitmodules` para trazabilidad completa de submódulos de ingeniería inversa.
- Documentación integral en `docs/research/README.md` alineada a `AGENTS.md`.
- Endpoint `GET /api/v1/sessions/{sessionId}/contacts` en la API REST y especificación OpenAPI 3.0.3.

### Modificado
- Migración de `docs/facebook-research/` a `docs/research/` cumpliendo la regla de `AGENTS.md` (línea 138).
- Corrección de la extracción de contactos y participantes en LightSpeed parser (`pkg/core/lightspeed/parser.go`) para `verifyContactRowExists`, `updateOrInsertThread` y `deleteThenInsertThread`.
- Normalización del parámetro de cuenta activa `av` en `pkg/features/thread/batch_data.go`.

### Fuentes de Investigación Consultadas y Registradas
- `docs/research/mautrix-meta` (última consulta: 2026-11-04)
- `docs/research/motofb` (última consulta: 2026-11-04)
- `docs/research/hermes` (última consulta: 2026-11-04)
- `docs/research/fbchat_v2` (última consulta: 2026-11-04)
- `docs/research/fbchat` (última consulta: 2026-11-04)
- `docs/research/fca-unofficial` (última consulta: 2026-11-04)
- `docs/research/fca-unofficial-tormairedusi` (última consulta: 2026-11-04)
- `docs/research/fca-unofficial-dongp06` (última consulta: 2026-11-04)
- `docs/research/facebook-chat-api` (última consulta: 2026-11-04)
- `docs/research/fb-messenger-e2ee` (última consulta: 2026-11-04)
- `docs/research/facebook-marketplace-mcp` (última consulta: 2026-11-04)
- `docs/research/purple-facebook` (última consulta: 2026-11-04)
- `docs/research/unzuckify` (última consulta: 2026-11-04)

---

## [2.3.3] - 2026-10-21

### Añadido
- Paridad completa del microservicio Go con `fbchat_v2` (Python).
- Servidor REST Chi v5, WebSocket hub y soporte de webhooks con firma HMAC.
- Almacenamiento seguro de sesiones cifrado con AES-256-GCM.
