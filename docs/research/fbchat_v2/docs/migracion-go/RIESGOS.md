# Gestión de Riesgos Técnicos y Mitigaciones — Migración fbchat_v2 a Go

La migración de un cliente y microservicio para una API no oficial como la de Facebook Messenger involucra retos técnicos críticos. Este documento cataloga los riesgos identificados durante la auditoría del código original y establece sus estrategias concretas de mitigación para la implementación en Go.

---

## 1. Matriz Resumen de Riesgos

| ID | Riesgo Técnico | Severidad | Probabilidad | Impacto | Estrategia de Mitigación |
|---|---|---|---|---|---|
| **R-01** | Huella TLS y detección de bots de Meta | Alta | Alta | Bloqueo temporal o permanente de cuentas (Checkpoints) | Rotación de User-Agents, cabeceras Client Hints realistas y emulación de huella HTTP/2 |
| **R-02** | Dependencia y complejidad del protocolo E2EE | Alta | Media | Fallo en mensajería cifrada de extremo a extremo | Soporte dual: integración nativa en Go o subproceso externo retrocompatible |
| **R-03** | Rotación o caducidad súbita de tokens (`fb_dtsg`) | Media | Alta | Fallos `403/400` en peticiones GraphQL y mutaciones | Re-extracción automática con refresco de sesión transparente |
| **R-04** | Vulnerabilidades SSRF en Webhooks salientes | Alta | Baja | Filtración de metadatos cloud (AWS/GCP) o escaneo de red interna | Validación estricta con bloqueo de IPs privadas y resolución DNS previa |
| **R-05** | Fugas de goroutines y consumo de memoria | Alta | Media | Degradación y caída del microservicio en producción | Uso riguroso de `context.Context`, canales cerrados y timeouts explícitos |
| **R-06** | Exposición de credenciales y cookies sensibles | Crítica | Media | Compromiso de cuentas de usuario en logs o endpoints | Redacción automática mediante regex y tipos `Secret` con enmascaramiento |
| **R-07** | Inconsistencia de secuencias en MQTT (`seq_id`) | Media | Media | Mensajes duplicados o pérdida de eventos en reconexiones | Persistencia y avance atómico de `seq_id` por sesión |

---

## 2. Análisis Detallado de Riesgos y Mitigaciones

### R-01: Detección de Bots y Huella de Red (Meta Antifraud)
- **Causa:** Facebook analiza activamente cabeceras de navegación, secuencias de orden en cabeceras HTTP, cifrados TLS (JA3/JA4) y patrones de tráfico. Si el cliente Go envía cabeceras por defecto de `Go-http-client`, la sesión es invalidada de inmediato.
- **Mitigación en Go:**
  - El paquete `pkg/core/utils` implementa la función `DefaultHeaders()` con rotación estricta de User-Agents recientes (Chrome sobre Windows, macOS, Linux).
  - Se replican con exactitud las cabeceras `sec-ch-ua`, `sec-ch-ua-mobile`, `sec-ch-ua-platform`, `sec-fetch-dest`, `sec-fetch-mode`, `sec-fetch-site` y `Origin`/`Referer`.
  - Configuración personalizada de `http.Transport` para respetar HTTP/2 y orden de cabeceras.

### R-02: Protocolo E2EE de Messenger (Cifrado Labyrinth)
- **Causa:** Messenger migró las conversaciones a cifrado de extremo a extremo obligatorio (basado en el protocolo Signal y almacenamiento Labyrinth). El proyecto Python original requería compilar un binario externo en Go y comunicarse por tuberías IPC.
- **Mitigación en Go:**
  - Dado que el puente original está escrito en Go (`mautrix-meta`), `fbreego` puede importar la lógica de cifrado directamente en su árbol de dependencias internas, evitando el uso de procesos hijos, tuberías y serialización redundante.
  - Para entornos que prefieran aislar el binario o donde se use una versión compilada preexistente, se conserva el cliente IPC `ProtocolManager` con interfaz idéntica a `_BridgeProcess`.

### R-03: Rotación y Caducidad de Tokens (`fb_dtsg`)
- **Causa:** El token CSRF `fb_dtsg` puede expirar si la sesión en el navegador cambia o si Facebook renueva credenciales de seguridad.
- **Mitigación en Go:**
  - `pkg/core/session.ExtractSession()` valida que los 5 campos obligatorios (`fb_dtsg`, `jazoest`, `sessionID`, `FacebookID`, `clientRevision`) estén presentes y sean válidos.
  - El cliente HTTP de operaciones detecta respuestas de error que indiquen token inválido y ejecuta un ciclo de re-obtención automática antes de fallar la operación.

### R-04: Ataques SSRF en Despachador de Webhooks
- **Causa:** Los usuarios pueden configurar URLs arbitrarias para recibir notificaciones por Webhook. Un atacante podría apuntar a `http://169.254.169.254/` (servicio de metadatos de AWS/GCP) o a servicios internos como `http://localhost:6379/` o `http://192.168.1.1/`.
- **Mitigación en Go:**
  - Replicación estricta del algoritmo `_is_ssrf_risk` en `pkg/service/dispatcher`:
  - Se resuelve el nombre de dominio a direcciones IP reales mediante `net.LookupIP()`.
  - Se comprueba cada IP contra listas negras:
    - Loopback: `127.0.0.0/8`, `::1/128`.
    - RFC 1918 (redes privadas): `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`.
    - Link-local y Cloud Metadata: `169.254.0.0/16`, `fe80::/10`.
    - Multicast y Broadcast.
  - Si la IP es riesgosa, la petición se rechaza con `ErrSSRFDetected` sin abrir la conexión.

### R-05: Fugas de Concurrencia y Goroutines
- **Causa:** En Go, lanzar `go func()` sin control dentro de conexiones WebSocket o listeners MQTT de larga duración puede acumular cientos de goroutines zombi si no se maneja la desconexión.
- **Mitigación en Go:**
  - Toda goroutine de servicio debe atarse al ciclo de vida de un `context.Context`.
  - Los loops de lectura y escritura de WebSocket utilizan `context.WithCancel` asociado al cierre del socket.
  - En pruebas unitarias y de integración se ejecuta `goleak` (`go.uber.org/goleak`) y el flag `-race` para garantizar cero fugas de goroutines y ausencia de carreras de datos.

### R-06: Filtración de Datos Sensibles en Logs
- **Causa:** Los mensajes y payloads contienen cookies (`c_user`, `xs`), tokens de acceso y contraseñas.
- **Mitigación en Go:**
  - Tipos especiales con serializador personalizado `String()` y `MarshalJSON()`:
    ```go
    type SecretString string
    func (s SecretString) String() string { return "<redacted>" }
    func (s SecretString) MarshalJSON() ([]byte, error) { return []byte("\"<redacted>\""), nil }
    ```
  - Filtro interceptor en el logger que aplica expresiones regulares de ofuscación equivalentes a `_redact_bridge_text`.
