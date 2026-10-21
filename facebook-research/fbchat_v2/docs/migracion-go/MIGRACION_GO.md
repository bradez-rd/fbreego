# Plan Maestro de Migración de fbchat_v2 a Go

## 1. Introducción y Propósito

El presente documento establece la guía técnica y estratégica para la migración completa del proyecto **`facebook-research/fbchat_v2`** (v2.3.3) desde Python hacia **Go (Golang)**. El objetivo primordial es lograr **paridad funcional del 100 %**, conservando todas las capacidades públicas e internas, estructuras de datos, mecanismos de autenticación, protocolos de comunicación en tiempo real y componentes de servicio, transformándolos en una solución idiomática, segura, altamente concurrente y libre de fugas de recursos en Go.

---

## 2. Principios Rectores de la Migración

1. **Regla de No Supresión:**
   Ninguna función, endpoint, estructura o parámetro existente en `fbchat_v2` debe ser omitido, simplificado arbitrariamente o sustituido sin trazabilidad documentada.
2. **Equivalencia Observable:**
   Las diferencias intrínsecas de lenguaje (como tipado estático frente a dinámico, o goroutines frente al event loop de Python) deben resolverse manteniendo idéntico el comportamiento observable externo: formatos de payload, cabeceras HTTP, secuencias de reconexión, firmas criptográficas y respuestas de error.
3. **Seguridad y Confidencialidad por Diseño:**
   Las cookies de sesión (`c_user`, `xs`, `fr`, `datr`), los tokens de acceso y las claves privadas nunca deben ser volcados en texto claro en logs o expuestos a través de DTOs públicos de la API. Se replican las técnicas de redacción y almacenamiento cifrado del original.
4. **Concurrencia Segura en Go:**
   Todo hilo o tarea asíncrona de Python (`asyncio.create_task`, `asyncio.Queue`, `threading.Thread`) se modela mediante **goroutines**, **canales con buffer tipados**, `context.Context` explícito para cancelación y `sync.Mutex` / `sync.RWMutex` para evitar condiciones de carrera (*race conditions*).
5. **Cero Fugas de Goroutines y Recursos:**
   Toda goroutine debe tener un ciclo de vida acotado y cancelable mediante `context.WithCancel` o `context.WithTimeout`. Las conexiones HTTP y WebSocket deben garantizar el cierre explícito de `resp.Body` y de descriptores de sockets.
6. **Manejo Explícito de Errores:**
   En lugar de propagar excepciones no controladas, Go utiliza retornos `(T, error)` con tipos de error personalizados (`Sentinel Errors` y tipos concretos con `errors.Is` y `errors.As`) que reflejan con exactitud los códigos y descripciones esperados.

---

## 3. Mapeo de Conceptos: Python vs. Go

| Paradigma en Python (`fbchat_v2`) | Equivalente Idiomático en Go (`fbreego`) | Justificación Técnica |
|---|---|---|
| `asyncio` event loop / `async def` / `await` | Goroutines ligeras + `context.Context` | Concurrencia nativa a nivel de runtime sin costes de context switching en espacio de usuario. |
| `pydantic.BaseModel` con validaciones | `struct` con tags `json:""` y validador explícito | Deserialización estricta mediante `encoding/json` y validaciones desacopladas con tipado fuerte. |
| `FastAPI` + decoradores de ruta | `net/http` estándar o router de alto rendimiento (`chi` / `gin`) | Menor consumo de memoria, compatibilidad nativa con la interfaz `http.Handler` de Go. |
| `dict[str, Any]` (ej. `dataFB`) | `struct DataFB` con campos tipados + `map[string]any` auxiliar | Seguridad en tiempo de compilación para tokens obligatorios y flexibilidad para campos variables. |
| `paho-mqtt` cliente MQTT | `github.com/eclipse/paho.golang` o conexión directa WebSocket | Cliente Go robusto para comunicación MQTT v3.1.1 sobre WebSocket TLS con edge-chat. |
| Subproceso puente E2EE | Módulo Go integrado nativo (`messagix` / Labyrinth) o cliente IPC Unix/NamedPipe | Go permite integrar nativamente el protocolo de cifrado de Messenger sin sobrecarga de procesos. |
| `cryptography.fernet.Fernet` | AES-256-GCM / HMAC-SHA256 estándar | Cifrado autenticado nativo en `crypto/aes` y `crypto/cipher` sin dependencias externas pesadas. |
| `httpx.AsyncClient` | `http.Client` con `http.Transport` personalizado y pool de conexiones | Reutilización de sockets HTTP/1.1 y HTTP/2 con soporte de proxies y TLS seguro. |

---

## 4. Hoja de Ruta y Fases de Migración

El proceso de migración se descompone en 11 fases estructuradas:

```text
[Fase 1: Auditoría] ➔ [Fase 2: Contratos & Compatibilidad] ➔ [Fase 3: Arquitectura Go Scaffolding]
         │
         ▼
[Fase 4: Autenticación & Sesiones] ➔ [Fase 5: Transporte & Parsers] ➔ [Fase 6: Mensajería Principal]
         │
         ▼
[Fase 7: Eventos & MQTT] ➔ [Fase 8: Red Social & Threads] ➔ [Fase 9: Servicio REST & Portal Web]
         │
         ▼
[Fase 10: Pruebas & Optimización] ➔ [Fase 11: Validación & Liberación Final]
```

---

## 5. Criterios de Calidad y Puertas de Aceptación (Quality Gates)

Para que el proyecto avance entre fases o sea considerado completo:

1. **Paridad de Pruebas Unitarias:** Cada módulo Go debe incluir pruebas unitarias con cobertura superior al 85 % en paquetes críticos (`session`, `parsers`, `crypto`, `messaging`).
2. **Detector de Carreras (`go test -race`):** Toda la suite de pruebas debe pasar sin advertencias del detector de carreras de Go.
3. **Análisis Estático Limpio:** Cero advertencias con `golangci-lint` (configurado con linters de seguridad, control de errcheck, gosimple y gosec).
4. **Verificación de Respuestas JSON:** Los esquemas JSON devueltos por el microservicio en Go deben ser 100 % idénticos a los producidos por la implementación FastAPI original (`code: "SUCCESS"`, `results`, `details`, etc.).
5. **No Regresión Funcional:** Verificación mediante pruebas end-to-end con mocks de Facebook y tests de integración controlados.
