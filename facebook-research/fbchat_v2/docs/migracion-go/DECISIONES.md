# Registros de Decisiones Arquitectónicas (ADR) — fbreego

Este documento registra las decisiones técnicas clave adoptadas para el diseño y migración de `facebook-research/fbchat_v2` a Go, detallando el contexto, las alternativas evaluadas y la justificación de la opción elegida.

---

## ADR-001: Estructura del Proyecto y Organización de Paquetes

- **Estado:** ACEPTADA
- **Contexto:** `fbchat_v2` en Python combinaba en la misma jerarquía una biblioteca cliente (`_core`, `_messaging`, `_features`) y un microservicio ejecutable (`service.py`, `portal.html`).
- **Decisión:** Adoptar el diseño canónico de Go:
  - `cmd/fbreego/`: Binario ejecutable del servicio y servidor web.
  - `pkg/`: Paquetes reutilizables e importables por terceros (`pkg/core`, `pkg/messaging`, `pkg/features`, `pkg/service`).
  - `web/`: Recursos estáticos incrustados.
- **Consecuencias:** Permite utilizar `fbreego` tanto como una librería Go independiente para bots como un microservicio REST/WebSocket autocontenido para arquitecturas políglotas.

---

## ADR-002: Elección del Router HTTP y Framework de Servicio

- **Estado:** ACEPTADA
- **Contexto:** El microservicio original utiliza FastAPI (Python). En Go se requiere una alternativa rápida, idiomática y compatible con la interfaz estándar `http.Handler`.
- **Alternativas consideradas:**
  1. `net/http` estándar (enrutador mejorado de Go 1.22+).
  2. `github.com/go-chi/chi/v5`: Router ligero, 100 % compatible con `net/http`, middleware encadenable, cero dependencias externas.
  3. `gin-gonic/gin`: Rápido pero introduce contexto propio desacoplado de `http.Handler`.
- **Decisión:** Utilizar **`chi v5`** (o el mux estándar de Go 1.22+ con middlewares estándar).
- **Justificación:** Chi mantiene compatibilidad total con la librería estándar de Go, ofrece soporte idiomático de subrouters, middlewares de autenticación Basic y CORS, y facilita la migración directa de los contratos de FastAPI.

---

## ADR-003: Librería para Manejo de WebSockets

- **Estado:** ACEPTADA
- **Contexto:** El servicio expone un endpoint `/ws` con tickets efímeros para streaming en tiempo real a clientes y al portal web.
- **Alternativas consideradas:**
  1. `gorilla/websocket`: Librería clásica pero con API antigua basada en lecturas bloqueantes sin soporte de `context.Context`.
  2. `github.com/coder/websocket` (antes `nhooyr.io/websocket`): API moderna diseñada para Go, cancelación estricta por `context.Context`, alto rendimiento y soporte de compresión.
- **Decisión:** Utilizar **`github.com/coder/websocket`**.
- **Justificación:** Su integración nativa con `context.Context` previene fugas de goroutines y simplifica el cierre coordinado de conexiones cuando una sesión se destruye o se detiene.

---

## ADR-004: Cliente MQTT para el Protocolo Edge-Chat de Messenger

- **Estado:** ACEPTADA
- **Contexto:** Messenger utiliza MQTT v3.1.1 sobre WebSocket seguro (`wss://edge-chat.facebook.com/chat`) para la entrega de mensajes en tiempo real y sincronización de deltas.
- **Alternativas consideradas:**
  1. `paho.mqtt.golang` (v1 clásica de Eclipse Paho).
  2. `github.com/eclipse/paho.golang` (v2 moderna, con tipado estricto y soporte de context).
  3. Implementación directa de framing MQTT sobre WebSocket.
- **Decisión:** Utilizar **`github.com/eclipse/paho.golang`** sobre el transporte WebSocket TLS.
- **Justificación:** Es la librería oficial mantenida por la fundación Eclipse para Go, garantiza cumplimiento de la especificación MQTT v3.1.1 y se integra limpiamente con flujos asíncronos.

---

## ADR-005: Esquema de Cifrado para Almacenamiento Local de Sesiones

- **Estado:** ACEPTADA
- **Contexto:** `service_sessions.py` en Python utiliza `cryptography.fernet.Fernet` (cifrado simétrico AES-128 en modo CBC con HMAC-SHA256 y timestamp).
- **Alternativas consideradas:**
  1. Implementar un decodificador Fernet en Go para compatibilidad binaria con archivos existentes generados por Python.
  2. Migrar a **AES-256-GCM** nativo (`crypto/aes`, `crypto/cipher`) con derivación de clave PBKDF2/Argon2 o SHA-256 de la variable `FBCHAT_SESSION_SECRET`.
- **Decisión:** Implementar **AES-256-GCM** como estándar principal de `fbreego` e incorporar un adaptador de lectura/migración transparente para payloads heredados en formato Fernet.
- **Justificación:** AES-256-GCM es el estándar moderno de cifrado autenticado en Go (AEAD), más rápido, con mayor nivel de seguridad y sin requerir firmas HMAC externas.

---

## ADR-006: Integración del Protocolo E2EE (Labyrinth / Messagix)

- **Estado:** ACEPTADA
- **Contexto:** El paquete Python ejecutaba un subproceso compilado en Go (`fbchat-bridge-e2ee`) comunicado por tuberías estándar stdin/stdout con JSON-RPC.
- **Decisión:** Implementar soporte dual:
  1. **Nativo en Go (Default):** Incorporación directa de las funciones de cifrado y protocolo de `mautrix-meta` / `messagix` dentro del mismo binario.
  2. **Subproceso Retrocompatible:** Capacidad opcional de invocar un binario externo mediante `FBCHAT_E2EE_BIN` para garantizar paridad estricta y permitir actualizaciones independientes del motor de cifrado.
- **Justificación:** Maximiza la velocidad y eficiencia de memoria eliminando la sobrecarga de subprocesos en la mayoría de despliegues, mientras protege la compatibilidad exacta con instalaciones existentes.

---

## ADR-007: Incrustación del Portal Web Administrativo SPA

- **Estado:** ACEPTADA
- **Contexto:** `portal.html` es una aplicación web completa de 68 KB que ofrece una interfaz de usuario para gestionar sesiones y monitorear eventos.
- **Decisión:** Incrustar el archivo directamente en el binario final mediante la directiva `//go:embed` de Go.
- **Justificación:** El binario compilado de `fbreego` será 100 % autocontenido (*single-binary deployment*), sin necesidad de distribuir archivos HTML o assets externos en el sistema operativo host.

---

## ADR-008: Implementación de 2FA TOTP

- **Estado:** ACEPTADA
- **Contexto:** `_core/_facebookLogin.py` depende de `pyotp` para derivar códigos de seis dígitos a partir de la semilla secreta de dos factores durante el login FB4A.
- **Decisión:** Implementar un generador TOTP RFC 6238 en `pkg/core/auth` utilizando exclusivamente la librería estándar (`crypto/hmac`, `crypto/sha1`, `encoding/base32`, `time`).
- **Justificación:** El algoritmo TOTP es directo y estándar; resolverlo con la librería estándar de Go elimina la necesidad de librerías de terceros y garantiza precisión temporal absoluta.
