# Fase 4 — Autenticación, Gestión de Sesiones y Almacenamiento

### A. Información General
- **Identificador:** FASE-04
- **Nombre de Fase:** Autenticación FB4A, Resolución de Sesión Web y Almacenamiento Cifrado
- **Objetivo:** Migrar los dos flujos de autenticación (login con credenciales y extracción de sesión por cookies), el generador de códigos 2FA y los sistemas de almacenamiento plano y cifrado multi-inquilino.
- **Prioridad:** Alta (P1 - Crítica)
- **Dependencias:** FASE-02, FASE-03.
- **Módulos originales involucrados:**
  - `fbchat_v2/_core/_session.py`
  - `fbchat_v2/_core/_storage.py`
  - `fbchat_v2/_core/_facebookLogin.py`
  - `fbchat_v2/service_sessions.py`
- **Archivos Go previstos:**
  - `pkg/core/session/session.go`
  - `pkg/core/storage/storage.go`
  - `pkg/core/storage/file.go`
  - `pkg/core/storage/env.go`
  - `pkg/core/auth/fb4a.go`
  - `pkg/core/auth/totp.go`
  - `pkg/service/sessions/record.go`
  - `pkg/service/sessions/registry.go`
  - `pkg/service/sessions/encrypted_store.go`
- **Estado inicial:** `PENDIENTE`

---

### B. Análisis del Código Original
- **Archivos y símbolos relevantes:**
  - `dataGetHome()`: Realiza GET a `https://www.facebook.com/`, busca en HTML tokens críticos mediante `_SPLIT_DATA_LIST`:
    - `fb_dtsg`: `DTSGInitialData",[],{"token":"` ... `"`
    - `fb_dtsg_ag`: `async_get_token":"` ... `"`
    - `jazoest`: `jazoest=` ... `"`
    - `hash`: `hash":"` ... `"`
    - `sessionID`: `sessionId":"` ... `"`
    - `FacebookID`: `"actorID":"` ... `"` (debe ser numérico)
    - `clientRevision`: `client_revision":` ... `,`
  - `loginFacebook`: POST a `https://b-graph.facebook.com/auth/login` simulando app Android FB4A con API Key fija y app token. Si recibe error 2FA (`1348162` o `1348023`), extrae `two_factor_first_step_metadata` y envía segundo POST con código TOTP.
  - `GetToken2FA()`: Genera token TOTP de 6 dígitos con ventana de 30 segundos usando HMAC-SHA1.
  - `EncryptedSessionStore`: Almacena cookies en disco por tenant en formato cifrado.
- **Riesgos específicos:**
  - Modificación de los patrones de búsqueda de tokens en el HTML de Facebook si cambian los scripts front-end de Meta.
  - Sincronización horaria crítica para la validez de los códigos TOTP 2FA.

---

### C. Plan de Implementación
- **Arquitectura propuesta:**
  - Implementar un extractor de tokens HTML robusto en Go utilizando `bytes.Index` o expresiones regulares optimizadas.
  - Implementar TOTP RFC 6238 nativo con `crypto/hmac` y `crypto/sha1`.
  - Implementar `EncryptedSessionStore` con AES-256-GCM y permisos `0600`.
- **Interfaces y estructuras Go:**
  ```go
  type SessionStorage interface {
      Load() (string, error)
      Save(cookies string) error
      Clear() error
  }

  type FB4AClient struct {
      httpClient *http.Client
      apiKey     string
      appToken   string
  }

  func (c *FB4AClient) Login(ctx context.Context, username, password, totpKey string) (*LoginResult, error)
  func ExtractSession(ctx context.Context, client *http.Client, cookies string) (*types.DataFB, error)
  ```

---

### D. Tareas Ejecutables
- [ ] Implementar la interfaz `SessionStorage` y las variantes `FileStorage` y `EnvStorage`.
- [ ] Implementar el extractor de sesión `ExtractSession` con parsing de tokens HTML y validación de campos obligatorios.
- [ ] Implementar el cliente `FB4ALogin` contra `b-graph.facebook.com`.
- [ ] Implementar el generador y validador de códigos 2FA TOTP (RFC 6238).
- [ ] Implementar `SessionRecord` y `SessionRegistry` con control de concurrencia mediante `sync.RWMutex`.
- [ ] Implementar `EncryptedSessionStore` utilizando AES-256-GCM con soporte de claves basadas en `FBCHAT_SESSION_SECRET`.
- [ ] Desarrollar suite de pruebas con fixtures HTML y respuestas de login simuladas.

---

### E. Matriz de Equivalencia
| Componente original | Referencia de origen | Equivalente Go | Prueba | Estado |
|---|---|---|---|---|
| `dataGetHome` | `_core/_session.py:dataGetHome` | `pkg/core/session.ExtractSession()` | Test con fixture HTML real | PENDIENTE |
| `loginFacebook` | `_core/_facebookLogin.py:loginFacebook` | `pkg/core/auth.FB4ALogin()` | Test con mock server FB4A | PENDIENTE |
| `GetToken2FA` | `_core/_facebookLogin.py:GetToken2FA` | `pkg/core/auth.GenerateTOTP()` | Test RFC 6238 vectores oficiales | PENDIENTE |
| `EncryptedSessionStore` | `service_sessions.py:EncryptedSessionStore` | `pkg/service/sessions.EncryptedStore` | Test de cifrado/descifrado | PENDIENTE |
| `SessionRegistry` | `service_sessions.py:SessionRegistry` | `pkg/service/sessions.Registry` | Test de concurrencia y validación | PENDIENTE |

---

### F. Pruebas y Aceptación
- Validación de generación de TOTP contra vectores de prueba RFC 6238.
- Prueba unitaria de `ExtractSession` ante HTML válido e HTML con sesión expirada.
- Prueba de concurrencia con 100 goroutines accediendo simultáneamente a `SessionRegistry`.

---

### G. Riesgos y Dependencias
- Manejo de proxies en el cliente HTTP de autenticación.

---

### H. Definición de Terminado
La fase se considerará terminada cuando los flujos de login, 2FA, extracción de tokens y almacenamiento cifrado pasen el 100 % de las pruebas unitarias sin condiciones de carrera.
