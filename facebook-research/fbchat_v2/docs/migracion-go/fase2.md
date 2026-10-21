# Fase 2 — Contratos y Compatibilidad

### A. Información General
- **Identificador:** FASE-02
- **Nombre de Fase:** Definición de Contratos de Datos y Matrices de Equivalencia Observable
- **Objetivo:** Formalizar todos los contratos de intercambio de datos (JSON DTOs, diccionarios internos, cabeceras y parámetros) garantizando equivalencia observable entre la implementación en Python y Go.
- **Prioridad:** Alta (P1 - Crítica)
- **Dependencias:** FASE-01 (Auditoría).
- **Módulos originales involucrados:**
  - `fbchat_v2/_core/_types.py`
  - `fbchat_v2/service.py` (Modelos Pydantic: `CookieInput`, `SessionCreateInput`, `TextMessageInput`, etc.)
  - `fbchat_v2/service_sessions.py` (`SessionRecord`)
  - `fbchat_v2/service_webhook.py` (`WebhookConfig`)
  - `fbchat_v2/service_media.py` (`MediaRecord`)
- **Archivos Go previstos:**
  - `pkg/core/types/session.go`
  - `pkg/core/types/responses.go`
  - `pkg/core/types/service_dto.go`
- **Estado inicial:** `PENDIENTE`

---

### B. Análisis del Código Original
- **Archivos y símbolos relevantes:**
  - `DataFB`: Diccionario que contiene `fb_dtsg`, `fb_dtsg_ag`, `jazoest`, `hash`, `sessionID`, `FacebookID`, `clientRevision`, `cookieFacebook`.
  - Respuestas del servicio FastAPI:
    - Éxito: `{"code": "SUCCESS", "message": str, "results": Any}`
    - Error: `{"code": str, "message": str, "details": Any, "traceId": Optional[str]}`
    - Validación: `{"code": "INVALID_REQUEST", "message": "The request payload is invalid.", "details": [{"field": str, "message": str}]}`
- **Comportamiento observable:**
  - Pydantic utiliza `populate_by_name=True` y `extra="forbid"` en inputs para rechazar campos no reconocidos (ej. `threadId` como alias de `thread_id`).
  - La clave de cookies `cookiesKey-ValueList` contiene la lista de cookies de sesión.
- **Riesgos específicos:** Incompatibilidad en nombres de campos JSON (`camelCase` vs `snake_case`) o en serialización de campos opcionales (`nil` vs omisión de clave).

---

### C. Plan de Implementación
- **Arquitectura propuesta:** Definición de structs Go con tags `json:"camelCase,omitempty"` y validador de campos requeridos.
- **Interfaces y estructuras Go:**
  ```go
  type DataFB struct {
      FbDtsg         string `json:"fb_dtsg"`
      FbDtsgAg       string `json:"fb_dtsg_ag"`
      Jazoest        string `json:"jazoest"`
      Hash           string `json:"hash"`
      SessionID      string `json:"sessionID"`
      FacebookID     string `json:"FacebookID"`
      ClientRevision string `json:"clientRevision"`
      CookieFacebook string `json:"cookieFacebook"`
  }

  type ServiceResponse[T any] struct {
      Code    string `json:"code"`
      Message string `json:"message"`
      Results T      `json:"results,omitempty"`
  }

  type ServiceErrorResponse struct {
      Code    string `json:"code"`
      Message string `json:"message"`
      Details any    `json:"details,omitempty"`
      TraceID string `json:"traceId,omitempty"`
  }
  ```

---

### D. Tareas Ejecutables
- [ ] Definir el tipo `DataFB` con validación estricta de campos obligatorios en Go.
- [ ] Definir los tipos de respuesta estándar `ServiceResponse` y `ServiceErrorResponse`.
- [ ] Modelar los DTOs de entrada del servicio (`SessionCreateInput`, `TextMessageInput`, `ReactionInput`, `UnsendInput`, `BlockInput`, `MediaUploadInput`, `MediaMessageInput`, `WebhookConfigInput`).
- [ ] Implementar un unmarshaler JSON estricto (`DisallowUnknownFields`) para replicar `extra="forbid"` de Pydantic.
- [ ] Crear fixtures de serialización/deserialización para verificar paridad byte a byte con Python.

---

### E. Matriz de Equivalencia
| Componente original | Referencia de origen | Equivalente Go | Prueba | Estado |
|---|---|---|---|---|
| `DataFB` | `_core/_types.py:DataFB` | `pkg/core/types.DataFB` | Test de serialización JSON | PENDIENTE |
| `SuccessResponse` | `_core/_types.py:SuccessResponse` | `pkg/core/types.SuccessResponse` | Test de paridad JSON | PENDIENTE |
| `ErrorResponse` | `_core/_types.py:ErrorResponse` | `pkg/core/types.ErrorResponse` | Test de paridad JSON | PENDIENTE |
| `TextMessageInput` | `service.py:TextMessageInput` | `pkg/core/types.TextMessageInput` | Test de validación y alias `threadId` | PENDIENTE |
| `SessionRecord` | `service_sessions.py:SessionRecord` | `pkg/service/sessions.SessionRecord` | Test de métodos `Public()` y `Detailed()` | PENDIENTE |

---

### F. Pruebas y Aceptación
- **Pruebas de Equivalencia:**
  - Comparación de cargas JSON generadas por Python con las generadas por Go ante los mismos datos de entrada.
  - Validación de que errores 422 en Go produzcan la misma estructura que `RequestValidationError` de FastAPI.

---

### G. Riesgos y Dependencias
- Manejo de aliases en Go: se debe asegurar que campos como `threadId` y `thread_id` se deserialicen correctamente según la convención del API.

---

### H. Definición de Terminado
La fase se considerará terminada cuando todos los structs de datos y DTOs estén definidos en Go y verificados con pruebas unitarias de serialización cruzada.
