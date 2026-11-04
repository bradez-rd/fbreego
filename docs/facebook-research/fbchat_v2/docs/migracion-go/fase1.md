# Fase 1 — Auditoría del Proyecto Original

### A. Información General
- **Identificador:** FASE-01
- **Nombre de Fase:** Auditoría Integral del Repositorio `fbchat_v2`
- **Objetivo:** Inspeccionar de forma recursiva y exhaustiva todos los módulos, dependencias, contratos de datos, flujos de red y supuestos del repositorio original en Python.
- **Prioridad:** Alta (P1 - Crítica)
- **Dependencias:** Ninguna (Fase inicial).
- **Módulos originales involucrados:**
  - Raíz: `fbchat_v2/` (`__init__.py`, `__main__.py`, `portal.html`, `service*.py`).
  - Núcleo: `fbchat_v2/_core/` (8 archivos `.py`).
  - Funcionalidades: `fbchat_v2/_features/` (19 archivos `.py` en `_facebook` y `_thread`).
  - Mensajería: `fbchat_v2/_messaging/` (13 archivos `.py`).
- **Archivos Go previstos:** N/A (Fase de auditoría, documentación y benchmarking).
- **Estado inicial:** `PENDIENTE`

---

### B. Análisis del Código Original
- **Archivos y símbolos relevantes:**
  - Total de 63 archivos físicos en `facebook-research/fbchat_v2`.
  - Dependencias de runtime identificadas en metadata: `httpx>=0.27.0`, `paho-mqtt>=1.6.1`, `pyotp>=2.9.0`, `requests>=2.32.0`, `fastapi<1.0.0`, `uvicorn[standard]`, `cryptography`, `websockets`.
- **Funcionalidades existentes:**
  - Autenticación dual: por cookies existentes (`dataGetHome`) o por usuario/contraseña/2FA vía endpoint de Android (`b-graph.facebook.com/auth/login`).
  - Extracción de tokens de formulario RelayModern: `fb_dtsg`, `jazoest`, `sessionID`, `FacebookID`, `clientRevision`.
  - Mensajería completa: texto, respuestas citadas, adjuntos multimedia, reacciones, borrado/unsend, edición mediante tareas LightSpeed.
  - Comunicación en tiempo real por MQTT sobre WebSocket (`edge-chat.facebook.com`).
  - Soporte de cifrado E2EE mediante puente compilado Go.
  - Microservicio HTTP REST con portal web SPA integrado y despacho de Webhooks con protección SSRF y firma HMAC-SHA256.
- **Comportamiento observable:**
  - Todas las respuestas JSON de Facebook que inician con `for (;;);` son limpiadas antes del parseo.
  - Todas las peticiones GraphQL requieren `__a=1`, `__req` en Base36 secuencial y cabeceras Client Hints realistas.
- **Casos especiales:**
  - Cuentas con 2FA activo devuelven error con subcódigos `1348162` o `1348023`.
  - Facebook devuelve HTML con JavaScript para LightSpeed en `/messages/t/`.
- **Riesgos específicos:** Detección de automatización si las cabeceras HTTP o la rotación de User-Agent difieren de las esperadas por los sistemas de Meta.

---

### C. Plan de Implementación
- **Arquitectura propuesta:** Creación de catálogo exhaustivo y pruebas de referencia (capturas de tráfico HTTP/JSON de muestra).
- **Secuencia ordenada de trabajo:**
  1. Recorrer y catalogar cada archivo `.py`.
  2. Extraer todos los endpoints de Facebook contactados.
  3. Mapear los tipos de datos de entrada y salida de cada función.
  4. Generar muestras de prueba (fixtures) con datos ficticios para alimentar los tests en Go.

---

### D. Tareas Ejecutables
- [ ] Catalogar la totalidad de archivos de `fbchat_v2` en el inventario funcional.
- [ ] Documentar los endpoints externos de Facebook (URLs, métodos, cabeceras requeridas).
- [ ] Documentar el protocolo de comunicación del puente E2EE (JSON-RPC v1).
- [ ] Documentar las rutas REST del microservicio FastAPI y sus esquemas Pydantic.
- [ ] Verificar que no existan dependencias no declaradas en el código fuente.

---

### E. Matriz de Equivalencia
| Componente original | Referencia de origen | Equivalente Go | Prueba | Estado |
|---|---|---|---|---|
| Mapeo de dependencias | `fbchat_v2-2.3.3.dist-info/METADATA` | `go.mod` dependencies | Comparación de capacidades | PENDIENTE |
| Endpoints de Facebook | Todo el código de `_core`, `_messaging`, `_features` | Constantes en `pkg/core/types` | Test de URLs | PENDIENTE |
| Catálogo de Errores | `_core/_types.py`, `_facebookLogin.py` | `pkg/core/types/errors.go` | Pruebas de tipos | PENDIENTE |

---

### F. Pruebas y Aceptación
- **Criterios de Aceptación:**
  - El 100 % de los archivos originales ha sido clasificado y registrado en `INVENTARIO_FUNCIONAL.md`.
  - No queda ninguna función pública o endpoint sin asignar a una fase posterior de migración.
  - Toda la documentación de auditoría ha sido aprobada.

---

### G. Riesgos y Dependencias
- Dependencia: Disponibilidad de acceso al código fuente de `facebook-research/fbchat_v2` (comprobada y verificada).

---

### H. Definición de Terminado
Esta fase se considerará completada cuando todos los artefactos de documentación de la auditoría (`README.md`, `MIGRACION_GO.md`, `INVENTARIO_FUNCIONAL.md`, `MATRIZ_COMPATIBILIDAD.md`, `ARQUITECTURA_GO.md`, `RIESGOS.md`, `DECISIONES.md`) estén creados y reflejen con exactitud el estado del repositorio auditado.
