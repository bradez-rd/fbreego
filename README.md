# fbreego

Servicio HTTP en Go para administrar sesiones y ejecutar operaciones compatibles con Facebook Messenger mediante endpoints web privados de Facebook.

> Este proyecto no es una API oficial de Meta. Los endpoints, tokens y formatos privados pueden cambiar sin aviso. Use una cuenta de prueba y revise los términos aplicables antes de desplegarlo.

## Estado de capacidades

Implementado y conectado:

- sesiones cifradas con AES-256-GCM;
- envío de texto, reacciones, unsend y mensajes con adjuntos previamente cargados;
- consultas sociales, threads y Marketplace disponibles en la API;
- staging local de medios;
- transporte realtime/MQTT para mensajes normales y ediciones, con clasificación directa de Meta AI (`threadType 201`);
- WebSocket para eventos generados por el servicio y recibidos del listener;
- webhooks HMAC con cola, reintentos y protección SSRF;
- OpenAPI en `/openapi.json` y Swagger UI en `/docs`.

No disponible todavía:

- backend criptográfico E2EE real.

La capacidad E2EE se reporta como no disponible y no devuelve éxitos simulados. El listener MQTT normaliza payloads realtime conocidos y publica `message.received`/`message.edited` hacia WebSocket y webhook; los cambios privados del protocolo de Meta pueden requerir adaptar nuevamente el decodificador.

## Requisitos y compilación

- Go 1.27.2 o la versión compatible indicada en `go.mod`.

```bash
go mod download
go test ./...
go build -o fbreego ./cmd/fbreego
```

## Configuración segura mínima

La autenticación está habilitada por defecto. El servicio requiere una clave de almacenamiento de al menos 32 caracteres y credenciales Basic completas.

```bash
export FBREEGO_SESSION_SECRET="replace-with-at-least-32-random-characters"
export FBREEGO_API_USERNAME="admin"
export FBREEGO_API_PASSWORD="replace-with-a-strong-password"
./fbreego --port 8000
```

Variables relevantes:

| Variable | Predeterminado | Uso |
|---|---:|---|
| `FBREEGO_PORT` | `8000` | Puerto HTTP; también se puede usar `--port`. |
| `FBREEGO_SESSIONS_DIR` | `.fbreego_sessions` | Sesiones, configuración de webhook y outbox de entrega cifrados. |
| `FBREEGO_MEDIA_DIR` | `.fbreego_media` | Archivos multimedia temporales. |
| `FBREEGO_SESSION_SECRET` | sin valor | Obligatoria, mínimo 32 caracteres. |
| `FBREEGO_API_AUTH_REQUIRED` | `true` | Solo use `false` explícitamente en desarrollo local. |
| `FBREEGO_API_USERNAME` | sin valor | Usuario de autenticación HTTP Basic. |
| `FBREEGO_API_PASSWORD` | sin valor | Contraseña de autenticación HTTP Basic. |
| `FBREEGO_API_CORS_ORIGINS` | sin orígenes | Lista separada por comas. `*` no habilita credenciales. |
| `FBREEGO_API_WS_ORIGINS` | sin orígenes | Patrones de origen WebSocket separados por comas. |
| `FBREEGO_TRAFFIC_ENABLED` | `false` | Activa el inspector de tráfico con vistas previas acotadas y redactadas. |
| `FBREEGO_ENABLE_E2E_TEST_ENDPOINT` | `false` | Expone la suite operativa `/api/v1/tests/e2e`; solo para entornos aislados. |
| `FBREEGO_WEBHOOK_ALLOWED_TARGETS` | sin valor | Orígenes administrados exactos, separados por comas, que pueden recibir webhooks aun si usan red privada o puerto no estándar. SocialManager lo configura con su propio `MAIN_SERVICE_URL`; los demás destinos continúan bajo la política anti-SSRF. |

El proceso se niega a inicializar el almacenamiento persistente si falta `FBREEGO_SESSION_SECRET`. Los identificadores `tenantId` están restringidos a un segmento seguro de 1–64 caracteres.

## Uso de la API

La lista autoritativa de rutas está en Swagger UI (`/docs`). Salvo `/healthz`, `/readyz`, `/openapi.json` y `/docs`, las rutas requieren HTTP Basic cuando la autenticación está activa.

Crear una sesión:

```bash
curl -u "$FBREEGO_API_USERNAME:$FBREEGO_API_PASSWORD" \
  -H "Content-Type: application/json" \
  -d '{
    "label": "cuenta-prueba",
    "tenantId": "default",
    "cookies": [
      {"name": "c_user", "value": "..."},
      {"name": "xs", "value": "..."}
    ]
  }' \
  http://localhost:8000/api/v1/sessions
```

Enviar texto:

```bash
curl -u "$FBREEGO_API_USERNAME:$FBREEGO_API_PASSWORD" \
  -H "Content-Type: application/json" \
  -d '{"threadId":"123456789","threadType":"user","text":"Hola"}' \
  http://localhost:8000/api/v1/sessions/SESSION_ID/messages/text
```

Para adjuntos, cargue primero el archivo en `/api/v1/media/upload` y pase los IDs devueltos a `/api/v1/sessions/{sessionId}/messages/media`. El servicio carga cada archivo a Facebook y enlaza los IDs de imagen, video, audio o archivo al mensaje final.

## WebSocket y webhooks

Solicite un ticket de un solo uso con `POST /api/v1/websocket-ticket` y conéctese a `/ws?ticket=...`. El ticket vence a los 30 segundos y se consume una sola vez.

Los webhooks se administran con:

- `GET`, `PUT` y `DELETE /api/v1/webhook`;
- `POST /api/v1/webhook/test`.

Los destinos se limitan a HTTP/HTTPS públicos en sus puertos estándar (80/443). Una instalación administrada puede autorizar orígenes internos mediante `FBREEGO_WEBHOOK_ALLOWED_TARGETS`; cada entrada debe coincidir exactamente en esquema, host y puerto. Los demás destinos privados, loopback, link-local, metadatos cloud, redirecciones inseguras y DNS internos permanecen bloqueados. Si se configura un secreto, cada entrega incluye `X-Hub-Signature-256`.

Los eventos se normalizan al contrato con puntos (`message.received`, `message.sent`, `message.edited`, `message.deleted`, `message.reaction` y `session.status`). Antes de entrar en la cola asíncrona, cada entrega se persiste cifrada en `webhook_outbox.enc`; los trabajos pendientes sobreviven reinicios y se reintentan con backoff exponencial. Al iniciar, el servicio también restaura de forma idempotente los listeners MQTT de las sesiones persistidas activas.

## Seguridad operativa

- No exponga el servicio directamente a Internet sin TLS y un proxy de confianza.
- No habilite el inspector de tráfico ni el endpoint E2E en producción salvo una necesidad explícita.
- Rote `FBREEGO_SESSION_SECRET` mediante un procedimiento de migración; cambiarla vuelve ilegibles los archivos ya cifrados.
- No registre ni comparta cookies `c_user`, `xs`, `fb_dtsg` u otros tokens de sesión.

## Licencia

El repositorio todavía no declara una licencia raíz. Debe elegirse y añadirse una antes de redistribuir el proyecto. Las referencias bajo `docs/research/` se usan para análisis comparativo; sus licencias no autorizan copiar código automáticamente al producto.
