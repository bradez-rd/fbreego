# Auditoría integral de seguridad y cumplimiento

**Proyecto:** `fbreego`  
**Fecha de auditoría:** 2026-10-21  
**Zona horaria:** America/La_Paz  
**Modalidad:** revisión de solo lectura, pruebas locales y contraste con fuentes oficiales  
**Tratamiento de secretos:** los valores de cookies, tokens y claves se omiten deliberadamente

## 1. Resumen ejecutivo

Se revisaron 70 archivos productivos (10.711 líneas), 31 archivos de pruebas, configuraciones, artefactos locales, historial Git, binarios y cuatro repositorios de investigación.

Conclusiones principales:

- No existe evidencia suficiente para identificar una causa confirmada o probable del bloqueo de una cuenta.
- Se identificaron factores **posibles** y técnicamente compatibles con controles de integridad de Meta:
  - automatización mediante endpoints web privados y GraphQL no documentado;
  - emulación inconsistente de navegadores y de la aplicación oficial FB4A;
  - ausencia de límites por cuenta y tratamiento específico de respuestas `403` y `429`;
  - exposición de dos sesiones reales en el historial remoto de Git.
- Las dos exportaciones de cookies pertenecen a sesiones diferentes; no se detectó reutilización de sus valores entre cuentas.
- La aplicación no estaba escuchando en el puerto 8000 y no había proxies ni variables de seguridad configuradas durante la auditoría.
- `go test ./...` y `go vet ./...` finalizaron correctamente, pero no cubren los riesgos principales de diseño y operación.
- El riesgo de recurrencia permanece **alto** mientras se utilicen cookies de cuentas personales contra endpoints privados.

Meta indica que utiliza límites, reconocimiento de patrones y bloqueos contra automatización no autorizada, y que puede aplicar medidas a cuentas asociadas. Esto hace que la arquitectura sea un factor posible, pero no demuestra que haya causado un bloqueo concreto.

Fuentes oficiales:

- https://www.facebook.com/help/463983701520800
- https://about.fb.com/news/2021/04/how-we-combat-scraping/

## 2. Alcance e inventario

### 2.1 Aplicación activa

- Módulo Go raíz: `github.com/bradez-rd/fbreego`.
- API HTTP/REST y WebSocket en `pkg/service/api/`.
- Sesiones y cifrado en `pkg/service/sessions/`.
- Clientes privados de Facebook en `pkg/core/`, `pkg/features/` y `pkg/messaging/`.
- Portal administrativo en `web/portal.html`.
- Binarios locales en `bin/`.

### 2.2 Referencias no integradas al módulo Go activo

- `facebook-research/facebook-marketplace-mcp`
- `facebook-research/fb-messenger-e2ee`
- `facebook-research/fca-unofficial`
- `facebook-research/mautrix-meta`
- distribución Python copiada en `facebook-research/fbchat_v2*`

Los cuatro primeros son repositorios Git anidados o entradas tipo gitlink. Sus dependencias no forman parte del binario Go raíz, salvo que se ejecuten por separado.

### 2.3 Estado observado

- No había proceso escuchando en TCP/8000.
- `HTTP_PROXY` y `HTTPS_PROXY` no estaban configurados.
- Las variables `FBCHAT_SESSION_SECRET`, `FBCHAT_API_USERNAME` y `FBCHAT_API_PASSWORD` no estaban definidas en la sesión auditada.
- No se encontraron archivos de log persistentes.
- Existían dos registros de sesión cifrados bajo `.fbchat_sessions/`.
- Los ACL de `.fbchat_sessions/` y `.fbchat_media/` concedían acceso únicamente al usuario local auditado.

## 3. Matriz de hallazgos

| ID | Severidad | Hallazgo | Relación con bloqueo |
|---|---|---|---|
| F-01 | Crítica | Cookies reales expuestas en historial remoto | Posible |
| F-02 | Crítica | Clave de cifrado predeterminada y conocida | Desconocida |
| F-03 | Alta | Uso de endpoints privados y suplantación de clientes oficiales | Posible |
| F-04 | Alta | Sin rate limiting por cuenta ni manejo de `403`, `429` o `Retry-After` | Posible |
| F-05 | Alta | Huella HTTP contradictoria y variable entre solicitudes | Posible |
| F-06 | Alta | Ciclo de vida y validación de cookies incompletos | Posible |
| F-07 | Alta | API en HTTP, Basic Auth y escucha en todas las interfaces | Posible indirecta |
| F-08 | Alta | Aislamiento de tenants únicamente nominal | Desconocida |
| F-09 | Alta condicional | Inspector y WebSocket pueden exponer contenido entre sesiones | Desconocida |
| F-10 | Media | CORS solo rechaza preflight; deja pasar solicitudes reales | Desconocida |
| F-11 | Media | Límites de entrada y sanitización de errores incompletos | Desconocida |
| F-12 | Alta | Go 1.27.2 afectado por una vulnerabilidad de `crypto/x509` | Desconocida |
| F-13 | Alta condicional | Dependencias vulnerables en repositorios de investigación | Desconocida |
| F-14 | Media | Binarios sin firma y construidos desde árbol modificado | Desconocida |
| F-15 | Media | Swagger remoto sin CSP/SRI y submódulos no reproducibles | Desconocida |
| F-16 | Media | Almacenamientos alternativos permiten cookies en texto plano | Desconocida |

## 4. Hallazgos detallados

### F-01 — Cookies reales publicadas en Git

**Ubicación:**

- `tests/default_cookies/ss1.json`
- `tests/default_cookies/ss2.json`
- commit de introducción: `81f42e4`

**Código o configuración implicada:** exportaciones completas de cookies del dominio `.facebook.com`.

**Evidencia:**

- Cada archivo contiene siete cookies.
- Están presentes `c_user`, `xs`, `fr`, `datr` y `sb`.
- Varias expiraban entre octubre de 2026 y noviembre de 2027.
- El commit `81f42e4` es alcanzable desde `origin/main`.
- Ignorar o eliminar los archivos del árbol actual no los retira de objetos y clones históricos.
- Los valores sensibles de ambos archivos son distintos; no se confirmó reutilización entre las dos cuentas.

**Impacto:** secuestro de sesión, acceso desde dispositivos o IP no reconocidos y pérdida de atribución sobre la actividad realizada con las cuentas.

**Severidad:** crítica.

**Relación con el bloqueo:** posible. Un tercero con acceso al repositorio podría haber usado las sesiones, pero no existen registros que lo confirmen.

**Corrección recomendada:**

1. Revocar ambas sesiones desde las opciones oficiales de Facebook.
2. Cambiar las contraseñas y revisar o renovar 2FA.
3. Revisar dispositivos, ubicaciones y sesiones reconocidas.
4. Rotar secretos del servicio relacionados.
5. Después de la revocación, purgar coordinadamente el historial remoto.

**Verificación:**

- Las cookies antiguas deben ser rechazadas.
- Ninguna referencia remota debe contener los objetos sensibles.
- Ejecutar un escáner de secretos sobre todas las referencias Git.

Fuentes oficiales:

- https://www.meta.com/safety/topics/safety-basics/tools/security/
- https://www.facebook.com/help/1216349518398524/

### F-02 — Clave de cifrado embebida

**Ubicación:** `pkg/service/runner/runner.go:35-37`.

**Configuración implicada:** cuando falta `FBCHAT_SESSION_SECRET`, el proceso instala una clave fija incluida en el código y el binario.

**Evidencia:** la literal está presente en el código. Los dos archivos `.enc` actuales no pudieron descifrarse usando esa clave, por lo que el impacto no está confirmado para los artefactos presentes.

**Impacto:** cualquier despliegue iniciado sin la variable cifra todas sus sesiones con una clave conocida y recuperable.

**Severidad:** crítica.

**Relación con el bloqueo:** desconocida; afecta principalmente confidencialidad y toma de cuentas.

**Corrección recomendada:** eliminar el fallback, exigir una clave aleatoria de 256 bits desde un gestor de secretos y añadir versionado y rotación de claves.

**Verificación:** el servicio debe negarse a arrancar sin secreto; una clave conocida no debe descifrar ninguna sesión de prueba.

### F-03 — Cliente no oficial y emulación de identidad

**Ubicaciones principales:**

- `pkg/core/auth/fb4a.go:18-21`
- `pkg/core/utils/headers.go:26-47`
- `pkg/messaging/send/sender.go:16`
- `pkg/features/social/posts.go:16-18`

**Descripción:** el proyecto utiliza endpoints privados de Mercury, GraphQL web y login FB4A, junto con tokens, IDs y cabeceras asociados a clientes que no pertenecen al proyecto.

**Evidencia:** comentarios y constantes indican expresamente que se emulan el navegador y la aplicación Android oficial. El login FB4A no tiene callers en el servicio actual, pero permanece exportado. Los endpoints web privados sí están conectados a rutas REST.

**Impacto:** incumplimiento potencial, rotura sin aviso y señales compatibles con automatización no autorizada.

**Severidad:** alta.

**Relación con el bloqueo:** posible.

**Corrección recomendada:** deshabilitar las operaciones hasta contar con una API oficial o autorización escrita de Meta. No intentar perfeccionar la imitación para evitar detección.

**Verificación:** inventario de red limitado a endpoints, permisos y credenciales oficialmente autorizados.

### F-04 — Ausencia de límites y clasificación de errores

**Ubicaciones representativas:**

- `pkg/messaging/reactions/reactions.go:58-69`
- `pkg/features/social/posts.go:131-155`
- `pkg/messaging/attachments/uploader.go:62-75`
- `pkg/service/api/routes.go:233-282`

**Evidencia:**

- No existe rate limiter por sesión, cuenta, IP u operación.
- Solicitudes concurrentes al API pueden producir llamadas concurrentes sobre la misma cuenta.
- Doce archivos realizan llamadas HTTP sin comprobar `StatusCode`.
- No se interpreta centralmente `429`, `403`, `Retry-After`, checkpoint o revocación.
- No se encontraron bucles automáticos infinitos de reintento contra Facebook.
- Los reintentos de webhook son tres, acotados y no se dirigen a Facebook.

**Impacto:** ráfagas accidentales, continuidad tras restricciones del proveedor y estados falsamente reportados como exitosos.

**Severidad:** alta.

**Relación con el bloqueo:** posible; no hay datos de volumen para confirmarlo.

**Corrección recomendada:** cola serializada por cuenta, límites configurables, circuito abierto ante `403` o checkpoint, respeto de `Retry-After` solo para APIs autorizadas y cero reintentos automáticos de mutaciones.

**Verificación:** pruebas con respuestas simuladas `401`, `403`, `429`, `5xx`, concurrencia y cancelación.

### F-05 — Headers y huella de cliente contradictorios

**Ubicación:** `pkg/core/utils/headers.go:9-47`.

**Evidencia:**

- El `User-Agent` cambia aleatoriamente entre Windows, macOS y Linux.
- `sec-ch-ua-platform` siempre declara Windows.
- `Sec-Fetch-User: ?1` se incluye en solicitudes tipo XHR/CORS.
- Se usa una versión fija de Chrome.
- `Host` se coloca en `Header`, aunque Go trata `Request.Host` como campo especial.
- Las cargas usan `Origin` y `Referer` de `upload.facebook.com`, sin demostrar que ese sea el contexto real de navegador.

**Impacto:** una misma sesión presenta una identidad técnica cambiante o imposible.

**Severidad:** alta por riesgo operativo y de cumplimiento.

**Relación con el bloqueo:** posible.

**Corrección recomendada:** utilizar un cliente oficial. Para diagnóstico, eliminar cabeceras inventadas y declarar una identidad técnica estable y honesta.

**Verificación:** snapshots de headers por operación sin contradicciones entre UA, client hints, origen y destino.

### F-06 — Gestión incompleta de cookies y sesiones

**Ubicaciones:**

- `pkg/core/types/service_dto.go:4-13`
- `pkg/service/sessions/registry.go:105-167`
- `pkg/service/sessions/record.go:25-37`
- `pkg/service/sessions/registry.go:253-272`

**Evidencia:**

- Solo se conservan nombre y valor de la cookie.
- Se pierden dominio, vencimiento, `Secure`, `HttpOnly` y `SameSite`.
- Se aceptan dos cookies cualesquiera, sin exigir explícitamente `c_user` y `xs`.
- No se impiden nombres duplicados ni dos registros para la misma cuenta.
- No se incorporan renovaciones recibidas mediante `Set-Cookie`.
- Eliminar una sesión borra el registro local, pero no revoca la sesión en Facebook.

**Impacto:** sesiones vencidas o duplicadas, pérdida de metadatos de seguridad y expectativa incorrecta de revocación.

**Severidad:** alta.

**Relación con el bloqueo:** posible por uso prolongado o incoherente de sesiones, sin prueba directa.

**Corrección recomendada:** validación estricta, deduplicación por cuenta, estados `blocked/stopped`, control de expiración y revocación manual u oficial al eliminar.

**Verificación:** pruebas de cookies duplicadas, expiradas, incompletas, cuenta repetida y revocación.

### F-07 — API administrativa por HTTP

**Ubicaciones:**

- `pkg/service/runner/runner.go:75-77`
- `pkg/service/runner/runner.go:123-137`
- `pkg/service/api/middleware.go:89-114`

**Evidencia:** escucha en `:puerto`, utiliza HTTP sin TLS y HTTP Basic. La autenticación es obligatoria por defecto y falla cerrada si faltan credenciales.

**Impacto:** si se expone sin proxy TLS, las credenciales administrativas y la información gestionada pueden ser interceptadas.

**Severidad:** alta.

**Relación con el bloqueo:** posible indirecta si produjo compromiso de sesiones; desconocida como causa directa.

**Corrección recomendada:** enlazar a `127.0.0.1` por defecto; usar TLS/mTLS o proxy autenticado; limitar intentos de autenticación.

**Verificación:** rechazo de HTTP claro, escaneo desde otra máquina y pruebas de fuerza bruta.

### F-08 — Aislamiento de tenants nominal

**Ubicaciones:**

- `pkg/service/sessions/registry.go:170-201`
- `pkg/service/api/routes.go:51-283`

**Evidencia:** `tenantId` separa directorios, pero una única credencial Basic puede listar, consultar y operar todas las sesiones. El filtro de tenant es proporcionado por el cliente.

**Impacto:** una credencial administrativa comprometida expone todas las cuentas y tenants.

**Severidad:** alta.

**Relación con el bloqueo:** desconocida.

**Corrección recomendada:** identidad y autorización por tenant, con asociación servidor-side entre sesión y principal.

**Verificación:** pruebas negativas cruzadas entre dos tenants.

### F-09 — Inspector y WebSocket

**Ubicaciones:**

- `pkg/service/traffic/middleware.go:75-98`
- `pkg/service/traffic/transport.go:48-93`
- `pkg/service/events/hub.go:95-112`

**Evidencia:**

- El inspector conserva hasta 64 KiB de cuerpos.
- La redacción protege tokens conocidos, pero no mensajes, perfiles u otros datos personales.
- Los eventos de tráfico no incluyen `sessionId`.
- El filtro WebSocket no restringe eventos sin `sessionId`, por lo que un suscriptor ligado a una sesión puede recibir eventos globales.
- `FBCHAT_TRAFFIC_ENABLED` estaba desactivado durante la auditoría.

**Impacto:** exposición cruzada de mensajes, respuestas y metadatos entre operadores o sesiones.

**Severidad:** alta cuando el inspector está habilitado.

**Relación con el bloqueo:** desconocida.

**Corrección recomendada:** registrar solo metadatos por defecto, omitir cuerpos, aplicar RBAC y exigir pertenencia de sesión en cada evento.

**Verificación:** dos WebSocket asociados a sesiones distintas no deben observar datos cruzados.

### F-10 — CORS incompleto

**Ubicación:** `pkg/service/api/middleware.go:127-158`.

**Evidencia:** un origen no autorizado recibe `403` en una solicitud `OPTIONS`, pero una solicitud real continúa hacia el handler.

**Impacto:** con autenticación desactivada, una solicitud simple cross-origin puede generar efectos aunque el navegador oculte la respuesta.

**Severidad:** media.

**Relación con el bloqueo:** desconocida.

**Corrección recomendada:** rechazar toda solicitud con `Origin` no permitido antes de ejecutar el handler; normalizar y validar esquema, host y puerto.

**Verificación:** pruebas `POST`, `DELETE` y WebSocket desde orígenes no autorizados.

### F-11 — Límites de entrada y errores

**Ubicaciones:**

- `pkg/service/api/handlers_session.go:31-41`
- `pkg/service/api/handlers_messages.go:18-39`
- `pkg/core/utils/parsers.go:48-54`
- `pkg/messaging/send/sender.go:146-152`

**Evidencia:** salvo el endpoint de media, los decodificadores JSON no tienen `MaxBytesReader`. Varios handlers devuelven `err.Error()` y pueden incluir fragmentos de respuestas del proveedor.

**Impacto:** consumo de memoria, exposición de información interna y almacenamiento de respuestas sensibles en diagnósticos.

**Severidad:** media.

**Relación con el bloqueo:** desconocida.

**Corrección recomendada:** límites globales, `DisallowUnknownFields`, un único documento JSON y errores públicos genéricos.

**Verificación:** cuerpos grandes, JSON concatenado y respuestas HTML con datos sensibles.

### F-12 — Vulnerabilidad de Go

**Ubicación:** `go.mod:3` y binarios locales.

**Evidencia:** `govulncheck` encontró `GO-2026-6809 / CVE-2026-111943` en `crypto/x509`, corregida en Go 1.27.3. No encontró vulnerabilidades alcanzables adicionales en los dos módulos Go externos.

**Impacto:** aceptación de determinados certificados intermedios que dejaron de considerarse confiables en casos específicos.

**Severidad:** alta.

**Relación con el bloqueo:** desconocida; no explica por sí sola una restricción de cuenta.

**Corrección recomendada:** compilar con Go 1.27.3 o posterior.

**Verificación:** repetir `govulncheck ./...`.

Fuentes:

- https://pkg.go.dev/vuln/GO-2026-6809
- https://go.dev/doc/devel/release

### F-13 — Dependencias vulnerables en referencias

**Resultados de `npm audit --package-lock-only`:**

| Proyecto | Total | Críticas | Altas | Medias | Bajas |
|---|---:|---:|---:|---:|---:|
| `facebook-marketplace-mcp` | 10 | 1 | 4 | 3 | 2 |
| `fca-unofficial` | 63 | 5 | 45 | 8 | 5 |
| `fb-messenger-e2ee` | 23 | 3 | 12 | 7 | 1 |

**Impacto:** alto si estos proyectos se ejecutan o incorporan al producto. No forman parte del binario Go actual.

**Relación con el bloqueo:** desconocida.

**Corrección recomendada:** mantenerlos aislados, no desplegarlos, retirar dependencias abandonadas y ejecutar auditorías en CI.

**Verificación:** auditorías sin vulnerabilidades críticas/altas aplicables y SBOM separado por proyecto.

### F-14 — Binarios no reproducibles ni firmados

**Artefactos:** `bin/fbreego.exe` y `bin/fbreego.exe~`.

**Evidencia:**

- Ambos fueron compilados con Go 1.27.2.
- Declaran revisión `81f42e4` con estado `dirty`.
- No tienen firma Authenticode.
- SHA-256 del binario principal: `250FBD2C9DAA05DB2417D3C0B05C1978119A44D6BE90651D45CC1C3951C51FC1`.

El hecho de que el commit contuviera cookies no implica que esas cookies estén embebidas en el ejecutable; sí están embebidas las constantes sensibles compiladas que usa el código.

**Severidad:** media.

**Corrección recomendada:** build limpio, reproducible, actualizado y firmado, acompañado por SBOM y procedencia.

**Verificación:** firma válida, hash publicado y coincidencia entre commit y fuentes limpias.

### F-15 — Dependencias remotas del portal y repositorios no reproducibles

**Ubicación:** `pkg/service/api/openapi.go:103-129`.

**Evidencia:** Swagger carga JavaScript y CSS desde `unpkg.com` sin SRI ni CSP. Además, cuatro gitlinks de `facebook-research/` no cuentan con `.gitmodules` en la raíz.

**Impacto:** dependencia de contenido remoto ejecutable y clonación incompleta del repositorio.

**Severidad:** media.

**Corrección recomendada:** empaquetar Swagger localmente, aplicar CSP, fijar acciones y dependencias por hash y declarar o retirar correctamente los submódulos.

**Verificación:** portal funcional sin acceso a CDN y clonación limpia reproducible.

### F-16 — Almacenamientos alternativos en texto plano

**Ubicaciones:**

- `pkg/core/storage/env.go:21-33`
- `pkg/core/storage/file.go:55-79`
- `pkg/core/storage/file.go:102-125`

**Evidencia:** existen implementaciones exportadas que guardan cookies directamente en una variable de entorno o JSON. El servicio principal utiliza el almacén cifrado, por lo que el riesgo está actualmente dormido.

**Impacto:** exposición accidental si un consumidor de la librería selecciona estos backends.

**Severidad:** media.

**Relación con el bloqueo:** desconocida.

**Corrección recomendada:** marcarlos explícitamente como solo pruebas, retirarlos de builds productivos o cifrar su contenido.

**Verificación:** análisis de callers y prueba que impida seleccionar almacenamiento plano en producción.

## 5. Controles positivos confirmados

- AES-256-GCM con nonce aleatorio para sesiones y webhook.
- ACL locales restrictivos en Windows y permisos `0600/0700` en POSIX.
- Validación de `tenantId` contra traversal.
- Protección SSRF de webhooks con validación DNS y de la IP efectivamente conectada.
- Redirecciones de webhook validadas y limitadas.
- Firma HMAC opcional para webhooks.
- Cola de webhook acotada y tres reintentos exponenciales.
- Tamaño de media limitado a 50 MiB.
- Autenticación administrativa obligatoria por defecto.
- Inspector de tráfico y endpoint E2E desactivados por defecto.
- No se utiliza `InsecureSkipVerify`.
- No se detectaron reintentos infinitos contra Facebook.

## 6. Análisis de posibles causas del bloqueo

No hay causas confirmadas ni probables con la evidencia disponible.

Factores posibles, en orden de relevancia técnica:

1. Uso automatizado de endpoints privados web, Mercury y GraphQL.
2. Cookies utilizadas fuera del navegador o dispositivo donde se originaron.
3. Identidad de cliente cambiante e internamente contradictoria.
4. Concurrencia y frecuencia sin límites por cuenta.
5. Exposición de sesiones en Git y posible uso desde otro dispositivo o IP.
6. Ejecución repetida de la suite E2E, si estuvo habilitada.

No existe evidencia actual de:

- proxy o VPN configurado;
- bucles infinitos de reintento contra Facebook;
- desactivación de validación TLS;
- reutilización de cookies entre las dos cuentas;
- invocación del login FB4A desde el servicio;
- volumen, timestamp o respuesta exacta inmediatamente anterior al bloqueo.

### Riesgo de recurrencia

- **Alto:** si se reanuda la automatización privada con cookies personales.
- **Medio:** si solo se rotan secretos, pero se mantienen endpoints privados y cabeceras simuladas.
- **Bajo:** únicamente tras migrar a integraciones autorizadas, aislar las cuentas y aplicar límites y respuesta a restricciones.

## 7. Plan de remediación priorizado

### Prioridad 0 — inmediata

1. Detener la automatización sobre las cuentas afectadas.
2. Revocar las dos sesiones expuestas.
3. Cambiar contraseñas y revisar 2FA, dispositivos y ubicaciones.
4. Rotar credenciales del API, secreto de sesiones y secretos HMAC.
5. Eliminar la clave de cifrado predeterminada.
6. Purgar coordinadamente el historial Git después de revocar las sesiones.
7. Actualizar a Go 1.27.3 y reconstruir desde un árbol limpio.
8. No reanudar endpoints privados hasta contar con una API oficial o autorización de Meta.

### Prioridad 1

- Ejecutor HTTP central con clasificación de errores.
- Límites y cola por cuenta.
- Aislamiento real por tenant.
- API solo local o detrás de TLS.
- Corregir CORS, límites de entrada y exposición de errores.
- Desactivar o rediseñar el inspector de tráfico.
- Gestión explícita de expiración y revocación de sesiones.

### Prioridad 2

- Añadir `govulncheck`, escáner de secretos y auditorías NPM al CI.
- Build firmado, SBOM y procedencia verificable.
- Logs persistentes y redactados con correlación por solicitud.
- Retirar código y repositorios de investigación innecesarios.
- Añadir licencia raíz y revisar compatibilidad de licencias.

## 8. Checklist de validación

- [ ] Cookies expuestas revocadas.
- [ ] Contraseñas y 2FA revisadas.
- [ ] Historial remoto sin archivos ni objetos sensibles.
- [ ] El servicio falla si falta el secreto de cifrado.
- [ ] Go actualizado y `govulncheck` limpio.
- [ ] Ninguna llamada a endpoints privados no autorizados.
- [ ] Límites por cuenta y pruebas de `403`/`429`.
- [ ] Sin reintentos automáticos de mutaciones.
- [ ] API accesible solo mediante TLS o localhost.
- [ ] Autorización por tenant probada.
- [ ] CORS rechaza preflight y solicitudes reales.
- [ ] WebSocket no cruza datos entre sesiones.
- [ ] Logs y errores no contienen cookies, tokens ni mensajes.
- [ ] Binario limpio, reproducible y firmado.
- [ ] `go test ./...`, `go vet ./...` y prueba de carreras aprobados.

La prueba de carreras quedó pendiente porque el entorno auditado tenía CGO deshabilitado.

## 9. Evidencias necesarias para apelación o revisión

Conservar, sin adjuntar secretos:

- Texto exacto, captura, fecha y zona horaria del aviso de bloqueo.
- ID de caso o mensaje del Support Inbox.
- Exportación oficial de actividad, seguridad e inicios de sesión.
- IP, dispositivo y ubicación de accesos reconocidos y no reconocidos.
- Códigos HTTP, `fbtrace_id`, timestamps y endpoint de las últimas operaciones.
- Volumen de solicitudes por minuto y concurrencia por cuenta.
- Logs del proxy, firewall y host.
- Commit `81f42e4`, fecha de revocación y prueba de purga.
- Versión y hash del binario ejecutado.
- Cronología factual: detección, detención, revocación y remediación.

No adjuntar cookies, tokens, contraseñas, claves 2FA ni cuerpos completos de mensajes.

Fuente oficial sobre acceso a actividad y datos de seguridad:

- https://www.facebook.com/help/windows-desktop/506794473544588/

## 10. Comandos de verificación ejecutados

```text
go test ./...                         PASS
go vet ./...                          PASS
go test -race ./pkg/service/...       NO EJECUTADO: CGO deshabilitado
govulncheck ./...                      1 vulnerabilidad alcanzable: GO-2026-6809
npm audit (referencias)                Hallazgos detallados en F-13
```

## 11. Limitaciones

- No se utilizaron cookies ni credenciales para consultar Facebook.
- No se realizaron intentos de evasión, bypass o reproducción del bloqueo.
- No se dispuso del aviso exacto de Meta, logs históricos de red ni actividad de inicio de sesión de las cuentas.
- La visibilidad pública o privada del repositorio remoto no se determinó; el commit sensible sí es alcanzable desde la referencia remota observada.
- La auditoría refleja el working tree existente el 2026-10-21, que contenía cambios sin confirmar previos a la revisión.
