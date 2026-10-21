# fbreego

Servicio HTTP en Go para administrar sesiones y ejecutar operaciones compatibles con Facebook Messenger mediante endpoints web privados de Facebook.

> Este proyecto no es una API oficial de Meta. Los endpoints, tokens y formatos privados pueden cambiar sin aviso. Use una cuenta de prueba y revise los términos aplicables antes de desplegarlo.

## Estado de capacidades

Implementado y conectado:

- sesiones cifradas con AES-256-GCM;
- envío de texto, reacciones, unsend y mensajes con adjuntos previamente cargados;
- consultas sociales, threads y Marketplace disponibles en la API;
- staging local de medios;
- WebSocket para eventos generados por el servicio;
- webhooks HMAC con cola, reintentos y protección SSRF;
- OpenAPI en `/openapi.json` y Swagger UI en `/docs`.

No disponible todavía:

- transporte realtime/MQTT para recibir mensajes desde Messenger;
- backend criptográfico E2EE real.

Esas capacidades se reportan como no disponibles; no devuelven éxitos simulados. Por ello, los WebSocket y webhooks reciben eventos producidos por operaciones locales, pero no mensajes entrantes desde Facebook.

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
export FBCHAT_SESSION_SECRET="replace-with-at-least-32-random-characters"
export FBCHAT_API_USERNAME="admin"
export FBCHAT_API_PASSWORD="replace-with-a-strong-password"
./fbreego --port 8000
```

Variables relevantes:

| Variable | Predeterminado | Uso |
|---|---:|---|
| `PORT` | `8000` | Puerto HTTP; también se puede usar `--port`. |
| `FBCHAT_SESSIONS_DIR` | `.fbchat_sessions` | Sesiones y configuración de webhook cifradas. |
| `FBCHAT_SESSION_SECRET` | sin valor | Obligatoria, mínimo 32 caracteres. |
| `FBCHAT_API_AUTH_REQUIRED` | `true` | Solo use `false` explícitamente en desarrollo local. |
| `FBCHAT_API_USERNAME` | sin valor | Usuario de autenticación HTTP Basic. |
| `FBCHAT_API_PASSWORD` | sin valor | Contraseña de autenticación HTTP Basic. |
| `FBCHAT_API_CORS_ORIGINS` | sin orígenes | Lista separada por comas. `*` no habilita credenciales. |
| `FBCHAT_API_WS_ORIGINS` | sin orígenes | Patrones de origen WebSocket separados por comas. |
| `FBCHAT_TRAFFIC_ENABLED` | `false` | Activa el inspector de tráfico con vistas previas acotadas y redactadas. |
| `FBCHAT_ENABLE_E2E_TEST_ENDPOINT` | `false` | Expone la suite operativa `/api/v1/tests/e2e`; solo para entornos aislados. |

El proceso se niega a inicializar el almacenamiento persistente si falta `FBCHAT_SESSION_SECRET`. Los identificadores `tenantId` están restringidos a un segmento seguro de 1–64 caracteres.

## Uso de la API

La lista autoritativa de rutas está en Swagger UI (`/docs`). Salvo `/healthz`, `/readyz`, `/openapi.json` y `/docs`, las rutas requieren HTTP Basic cuando la autenticación está activa.

Crear una sesión:

```bash
curl -u "$FBCHAT_API_USERNAME:$FBCHAT_API_PASSWORD" \
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
curl -u "$FBCHAT_API_USERNAME:$FBCHAT_API_PASSWORD" \
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

Los destinos se limitan a HTTP/HTTPS públicos en sus puertos estándar (80/443). Se bloquean IPs privadas, loopback, link-local, metadatos cloud, redirecciones inseguras y DNS que resuelva a redes internas. Si se configura un secreto, cada entrega incluye `X-Hub-Signature-256`.

## Seguridad operativa

- No exponga el servicio directamente a Internet sin TLS y un proxy de confianza.
- No habilite el inspector de tráfico ni el endpoint E2E en producción salvo una necesidad explícita.
- Rote `FBCHAT_SESSION_SECRET` mediante un procedimiento de migración; cambiarla vuelve ilegibles los archivos ya cifrados.
- No registre ni comparta cookies `c_user`, `xs`, `fb_dtsg` u otros tokens de sesión.

## Licencia

El repositorio todavía no declara una licencia raíz. Debe elegirse y añadirse una antes de redistribuir el proyecto. Las referencias bajo `facebook-research/` se usan para análisis comparativo; sus licencias no autorizan copiar código automáticamente al producto.
