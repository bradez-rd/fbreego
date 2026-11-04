# Fase 5 — Comunicación, Transporte HTTP y Parsers de Protocolo

### A. Información General
- **Identificador:** FASE-05
- **Nombre de Fase:** Transporte HTTP, Ensamblador de Formularios RelayModern y Parsers LightSpeed
- **Objetivo:** Implementar la capa de transporte HTTP de alto rendimiento, rotación de cabeceras/huellas, generadores de identificadores únicos, constructores de formularios GraphQL de Facebook y decodificadores JSON especiales.
- **Prioridad:** Alta (P1 - Crítica)
- **Dependencias:** FASE-02, FASE-04.
- **Módulos originales involucrados:**
  - `fbchat_v2/_core/_http.py`
  - `fbchat_v2/_core/_utils.py`
  - `fbchat_v2/service_provider.py` (Lógica de extracción LightSpeed)
- **Archivos Go previstos:**
  - `pkg/core/httputil/client.go`
  - `pkg/core/utils/headers.go`
  - `pkg/core/utils/ids.go`
  - `pkg/core/utils/forms.go`
  - `pkg/core/utils/parsers.go`
  - `pkg/core/lightspeed/parser.go`
- **Estado inicial:** `PENDIENTE`

---

### B. Análisis del Código Original
- **Archivos y símbolos relevantes:**
  - `Headers()`: Configura `Host`, `Origin`, `Referer`, `Sec-Fetch-*`, `Sec-CH-UA` y rota User-Agents entre Windows, macOS y Linux.
  - `formAll()`: Construye el payload común de Facebook:
    ```python
    {
        "fb_dtsg": dataFB["fb_dtsg"],
        "jazoest": dataFB["jazoest"],
        "__a": 1,
        "__user": dataFB["FacebookID"],
        "__req": str_base(next(_REQUEST_COUNTER), 36),
        "__rev": dataFB["clientRevision"],
        "av": dataFB["FacebookID"],
        "fb_api_caller_class": "RelayModern",
        "fb_api_req_friendly_name": FBApiReqFriendlyName,
        "server_timestamps": "true",
        "doc_id": str(docID)
    }
    ```
  - `str_base(number, 36)`: Genera identificador de secuencia de petición incremental en Base36 minúscula.
  - `parse_json_response()`: Remueve el prefijo anti-hijacking `for (;;);` antes de decodificar JSON.
  - `gen_threading_id()`: Genera ID numérico de 64 bits ensamblando timestamp en milisegundos en binario concatenado con 22 bits aleatorios.
  - `fetch_lightspeed_data()`: Descarga `/messages/t/`, extrae llamadas LightSpeed y mapea threads.
- **Riesgos específicos:** Si el formato del formulario RelayModern o el cálculo de `jazoest`/`__req` diverge, Facebook responde con error `400 Bad Request` o silent drops.

---

### C. Plan de Implementación
- **Arquitectura propuesta:**
  - Implementar un generador de secuencias atómico concurrente (`atomic.Uint64`) para `__req`.
  - Implementar pool de conexiones HTTP reutilizables en `pkg/core/httputil`.
  - Crear parser de respuestas LightSpeed con decodificación de payloads de base de datos embebidos.
- **Interfaces y estructuras Go:**
  ```go
  type HTTPTransport interface {
      PostForm(ctx context.Context, url string, form url.Values, cookies string) (*http.Response, error)
      Get(ctx context.Context, url string, cookies string) (*http.Response, error)
  }

  type RequestSequence struct {
      counter atomic.Uint64
  }

  func (s *RequestSequence) NextBase36() string
  func CleanJSONPayload(data []byte) ([]byte, error)
  func GenThreadingID() string
  func BuildFormAll(fb *types.DataFB, friendlyName string, docID string) url.Values
  ```

---

### D. Tareas Ejecutables
- [ ] Implementar cliente HTTP reutilizable con Keep-Alive y configuración de proxies.
- [ ] Implementar rotación de cabeceras Client Hints y User-Agents en `DefaultHeaders()`.
- [ ] Implementar `str_base` y generador de secuencia Base36 seguro para hilos (`sync/atomic`).
- [ ] Implementar generador de identificadores `GenThreadingID()`, `GenerateClientID()`, `GenerateSessionID()`.
- [ ] Implementar `BuildFormAll()` con soporte de campos RelayModern.
- [ ] Implementar decodificador JSON tolerante que suprima `for (;;);` y decodifique números grandes como strings o `json.Number`.
- [ ] Implementar extractor y parser de llamadas RPC LightSpeed (`_parse_lightspeed_data_from_html`).

---

### E. Matriz de Equivalencia
| Componente original | Referencia de origen | Equivalente Go | Prueba | Estado |
|---|---|---|---|---|
| `Headers` | `_core/_utils.py:Headers` | `pkg/core/utils.DefaultHeaders()` | Test de cabeceras esperadas | PENDIENTE |
| `str_base` | `_core/_utils.py:str_base` | `pkg/core/utils.ToBase36()` | Test con valores enteros conocidos | PENDIENTE |
| `formAll` | `_core/_utils.py:formAll` | `pkg/core/utils.BuildFormAll()` | Test de estructura del formulario | PENDIENTE |
| `parse_json_response` | `_core/_utils.py:parse_json_response` | `pkg/core/utils.ParseFacebookJSON()` | Test con prefijo `for (;;);` | PENDIENTE |
| `gen_threading_id` | `_core/_utils.py:gen_threading_id` | `pkg/core/utils.GenThreadingID()` | Test de longitud y rango numérico | PENDIENTE |
| `fetch_lightspeed_data` | `service_provider.py:fetch_lightspeed_data` | `pkg/core/lightspeed.FetchData()` | Test con fixture de /messages/t/ | PENDIENTE |

---

### F. Pruebas y Aceptación
- Comprobación matemática exacta de `GenThreadingID()`: debe producir enteros positivos de hasta 19 dígitos.
- Verificación de decodificación limpia de respuestas JSON con y sin prefijo `for (;;);`.
- Prueba de concurrencia: múltiples goroutines solicitando números Base36 concurrentemente sin colisiones.

---

### G. Riesgos y Dependencias
- Manejo de memoria al parsear grandes documentos HTML de LightSpeed.

---

### H. Definición de Terminado
La fase se considerará terminada cuando todo el conjunto de transporte HTTP, constructores de formularios y utilidades de parsing pasen sus pruebas unitarias.
