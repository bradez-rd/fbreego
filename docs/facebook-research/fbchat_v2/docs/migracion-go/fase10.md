# Fase 10 — Pruebas Automatizadas, Optimización y Análisis de Rendimiento

### A. Información General
- **Identificador:** FASE-10
- **Nombre de Fase:** Suite de Pruebas de Regresión, Detección de Carreras y Optimización
- **Objetivo:** Construir una suite integral de pruebas unitarias y de integración con servidores mock de Facebook, verificar la total ausencia de fugas de memoria o goroutines con `-race` y `goleak`, y optimizar las rutas calientes de procesamiento (*hot paths*).
- **Prioridad:** Alta (P1 - Crítica)
- **Dependencias:** FASE-03 a FASE-09.
- **Módulos originales involucrados:**
  - Todos los módulos del proyecto y especificaciones de tests (`pytest` en Python).
- **Archivos Go previstos:**
  - `testutil/mocks/facebook_mock.go`
  - `testutil/mocks/graphql_mock.go`
  - `testutil/mocks/mqtt_broker_mock.go`
  - Archivos `*_test.go` en todos los paquetes `pkg/`
  - `pkg/core/utils/benchmark_test.go`
- **Estado inicial:** `PENDIENTE`

---

### B. Análisis del Código Original
- **Archivos y símbolos relevantes:**
  - En el repositorio original, muchas pruebas dependían de variables de entorno reales o ejecuciones directas.
  - Se requiere diseñar una suite determinista y reproducible sin requerir credenciales reales de Facebook en CI/CD.
- **Riesgos específicos:**
  - Condiciones de carrera ocultas en el acceso a mapas de sesiones o clientes WebSocket.
  - Fugas de goroutines en reconexiones cíclicas de listeners.

---

### C. Plan de Implementación
- **Arquitectura propuesta:** Servidores de prueba locales con `httptest.Server` que reproducen los contratos, cabeceras y respuestas de Facebook (`for (;;);`, RelayModern, GraphQLBatch).
- **Herramientas de validación:**
  - `go test -race -v ./...`
  - `go test -coverprofile=coverage.out ./...` (objetivo: > 85 % cobertura en paquetes core)
  - `go.uber.org/goleak` en todos los tests de tiempo real y servicios.
  - `go test -bench=. -benchmem ./...` para análisis de asignaciones de memoria (*allocs/op*).

---

### D. Tareas Ejecutables
- [ ] Construir el servidor mock `FacebookMockServer` con soporte de endpoints REST y GraphQL RelayModern.
- [ ] Construir el broker mock MQTT para pruebas locales de suscripciones y envío de deltas.
- [ ] Escribir pruebas unitarias exhaustivas para `pkg/core/session` con fixtures HTML válidos, expirados y malformados.
- [ ] Escribir pruebas unitarias para `pkg/service/dispatcher` validando el bloqueo de 50 variantes de URLs para SSRF.
- [ ] Ejecutar el detector de carreras (`go test -race ./...`) corrigiendo cualquier sincronización deficiente.
- [ ] Integrar `goleak` en las pruebas de WebSocket y listeners para confirmar que no queden goroutines huérfanas tras el cierre.
- [ ] Escribir benchmarks para `GenThreadingID()`, `ParseFacebookJSON()` y `BuildFormAll()`, optimizando asignaciones de memoria a cero o mínimas.

---

### E. Matriz de Equivalencia
| Componente original | Referencia de origen | Equivalente Go | Prueba | Estado |
|---|---|---|---|---|
| Suite de pruebas pytest | `tests/` | Suite `*_test.go` | `go test ./...` | PENDIENTE |
| Verificación de concurrencia | Asincronía Python | `go test -race` | Detector de carreras Go | PENDIENTE |
| Verificación de fugas | Garbage collector Python | `goleak.VerifyNone()` | Test de fuga de goroutines | PENDIENTE |

---

### F. Pruebas y Aceptación
- Cobertura de pruebas superior al 85 % en paquetes críticos.
- 0 condiciones de carrera reportadas por el compilador con `-race`.
- 0 fugas de goroutines detectadas tras completar los tests de ciclo de vida.
- Reducción medible del consumo de memoria y CPU respecto a la versión Python original.

---

### G. Riesgos y Dependencias
- Necesidad de emular con fidelidad las respuestas GraphQL de Facebook en los mocks.

---

### H. Definición de Terminado
La fase se considerará terminada cuando toda la suite de pruebas se ejecute en verde, sin advertencias de concurrencia y cumpliendo los objetivos de cobertura y rendimiento.
