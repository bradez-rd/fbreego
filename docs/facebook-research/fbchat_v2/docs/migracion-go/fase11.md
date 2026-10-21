# Fase 11 — Validación Final, Empaquetado y Estrategia de Liberación

### A. Información General
- **Identificador:** FASE-11
- **Nombre de Fase:** Validación Integral de Paridad, Empaquetado Binario y Estrategia de Transición
- **Objetivo:** Verificar la equivalencia funcional del 100 % entre la implementación en Python y la solución en Go mediante pruebas de integración extremo a extremo, compilar el binario ejecutable autocontenido y establecer el plan de reemplazo operativo del proyecto original.
- **Prioridad:** Alta (P1 - Crítica)
- **Dependencias:** FASE-01 a FASE-10.
- **Módulos originales involucrados:**
  - Todo el repositorio `facebook-research/fbchat_v2`.
- **Archivos Go previstos:**
  - `cmd/fbreego/main.go`
  - `Makefile` / `Taskfile.yml`
  - `Dockerfile` (imagen distroless o alpine de < 25 MB)
- **Estado inicial:** `PENDIENTE`

---

### B. Análisis del Código Original
- **Punto de corte operativo:**
  - La versión Python requería un entorno virtual con múltiples dependencias pesadas (`httpx`, `paho-mqtt`, `fastapi`, `uvicorn`, `cryptography`, wheel de `bridge-e2ee`).
  - La solución Go en `fbreego` consolidará todo en un **único archivo binario ejecutable estático** (`fbreego.exe` en Windows o `fbreego` en Linux), conteniendo el servidor HTTP, el motor MQTT, el portal web incrustado y el almacenamiento cifrado.
- **Variables de entorno soportadas y preservadas:**
  - `FBCHAT_API_USERNAME`: Usuario para autenticación básica de la API.
  - `FBCHAT_API_PASSWORD`: Contraseña para autenticación básica de la API.
  - `FBCHAT_API_AUTH_REQUIRED`: `true`/`false` para activar la protección de la API.
  - `FBCHAT_SESSION_SECRET`: Semilla o clave maestra para cifrado de sesiones y webhooks.
  - `FBCHAT_PERSIST_SESSIONS`: Bandera para habilitar almacenamiento en disco (`true`/`false`).
  - `FBCHAT_E2EE_BIN`: Ruta a binario externo opcional para compatibilidad con puentes legados.
  - `FBCHAT_E2EE_REQUIRED`: Exigir operatividad de E2EE para considerar el servicio listo (`/readyz`).
  - `PORT`: Puerto de escucha del servidor (por defecto `8000`).

---

### C. Plan de Implementación
- **Arquitectura de entrega:**
  - Compilación estática con flags de optimización y reducción de símbolos:
    ```bash
    go build -ldflags="-s -w -X main.version=2.3.3" -o bin/fbreego ./cmd/fbreego
    ```
  - Construcción de contenedor Docker multi-stage con base `gcr.io/distroless/static-debian12`.
- **Estrategia de transición (*Cutover*):**
  1. **Validación paralela:** Ejecución de ambas soluciones en paralelo respondiendo a los mismos payloads en entorno de staging.
  2. **Migración de datos:** Utilidad CLI para migrar sesiones existentes de formato Fernet a formato AES-256-GCM.
  3. **Despliegue final:** Sustitución directa del proceso sin modificaciones en clientes que consuman la API REST o WebSocket.

---

### D. Tareas Ejecutables
- [ ] Implementar el punto de entrada CLI en `cmd/fbreego/main.go` con lectura de variables de entorno y soporte de flags CLI.
- [ ] Configurar el apagado limpio (*graceful shutdown*) capturando señales de interrupción `SIGINT` y `SIGTERM`.
- [ ] Implementar herramienta de migración de sesiones cifradas heredadas de Python a Go.
- [ ] Crear `Dockerfile` optimizado multi-etapa para producción.
- [ ] Ejecutar batería de pruebas de humo (*smoke tests*) contra todos los endpoints HTTP y WebSocket.
- [ ] Validar la visualización y funcionalidad completa del portal SPA (`portal.html`) servido por el binario compilado.
- [ ] Actualizar la documentación de despliegue y guías de uso de `fbreego`.

---

### E. Matriz de Equivalencia
| Componente original | Referencia de origen | Equivalente Go | Prueba | Estado |
|---|---|---|---|---|
| CLI runner | `fbchat_v2/__main__.py:main` | `cmd/fbreego/main.go` | Prueba de arranque binario | PENDIENTE |
| Variables de entorno | `service.py:readiness_report` | `pkg/service/api.Config` | Test de paridad de configuración | PENDIENTE |
| Despliegue en contenedor | Entorno Python | Dockerfile distroless | Test de contenedor Docker | PENDIENTE |

---

### F. Pruebas y Aceptación
- **Criterios de Aceptación Cuantitativos:**
  - Tamaño del binario compilado: < 35 MB (sin UPX) o < 15 MB (comprimido).
  - Uso de memoria en reposo: < 25 MB de RSS (frente a los ~120 MB del runtime de Python + FastAPI).
  - Tiempo de arranque en frío: < 50 ms (frente a los ~1.5 s de Python).
  - 100 % de paridad funcional en las 80 capacidades catalogadas en `MATRIZ_COMPATIBILIDAD.md`.

---

### G. Riesgos y Dependencias
- Verificación de compatibilidad de sesiones almacenadas previamente por el servicio en Python.

---

### H. Definición de Terminado
La fase se considerará terminada y el proyecto formalmente liberado cuando:
- El binario de Go compile limpiamente para Windows y Linux.
- Se hayan validado con éxito todas las fases previas (Fase 1 a Fase 10).
- La documentación técnica refleje fielmente la operación del nuevo software.
- Se haya comprobado la paridad observable en el consumo del portal web y la API REST.
