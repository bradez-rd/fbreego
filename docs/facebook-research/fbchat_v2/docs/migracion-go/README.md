# Plan Maestro de Migración: fbchat_v2 a Go (fbreego)

Bienvenido a la documentación oficial y plan de migración integral de **`facebook-research/fbchat_v2`** hacia **Golang**.

Este conjunto de documentos define la auditoría técnica exhaustiva, el inventario funcional, la matriz de compatibilidad, la arquitectura de destino en Go y la descomposición paso a paso en **11 fases ejecutables**, diseñadas específicamente para guiar la reimplementación sin pérdida de capacidades, con máxima fidelidad operativa y respetando los estándares idiomáticos de Go.

---

## 🧭 Mapa de Documentación

| Documento | Descripción |
|---|---|
| [MIGRACION_GO.md](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/MIGRACION_GO.md) | Guía maestra de la migración: principios rectores, orden de ejecución y criterios de paridad. |
| [INVENTARIO_FUNCIONAL.md](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/INVENTARIO_FUNCIONAL.md) | Catálogo minucioso de todos los archivos (63), módulos, símbolos, funciones, clases y rutas HTTP de la librería original. |
| [MATRIZ_COMPATIBILIDAD.md](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/MATRIZ_COMPATIBILIDAD.md) | Matriz global de trazabilidad con identificadores únicos (`F-001` a `F-080`), asignación a fase y estado. |
| [ARQUITECTURA_GO.md](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/ARQUITECTURA_GO.md) | Diseño arquitectónico del nuevo módulo Go: paquetes, interfaces, modelos de concurrencia, gestión de errores y ciclo de vida. |
| [RIESGOS.md](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/RIESGOS.md) | Análisis de riesgos técnicos (detección de bots, cifrado E2EE, seguridad SSRF, fugas de memoria) y mitigaciones. |
| [DECISIONES.md](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/DECISIONES.md) | Registros de Decisión Arquitectónica (ADR) con justificaciones técnicas de cada patrón adoptado. |

---

## 🚀 Fases de Ejecución

Cada fase está autocontenida y estructurada con las secciones obligatorias **A a H**, listas de tareas en formato checklist y criterios medibles de aceptación:

1. [fase1.md — Auditoría del proyecto original](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/fase1.md)
2. [fase2.md — Contratos y compatibilidad](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/fase2.md)
3. [fase3.md — Arquitectura del proyecto Go (Scaffolding & Core Foundation)](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/fase3.md)
4. [fase4.md — Autenticación y sesiones (FB4A, 2FA, dataGetHome y Storage)](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/fase4.md)
5. [fase5.md — Comunicación y transporte (HTTP, RelayModern, LightSpeed, Parsers)](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/fase5.md)
6. [fase6.md — Mensajería (Envío, Adjuntos, Respuestas, Edición, Unsend, Reacciones, Notas, Temas)](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/fase6.md)
7. [fase7.md — Eventos y tiempo real (MQTT edge-chat, Sync Sequences, WebSocket Hub)](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/fase7.md)
8. [fase8.md — Multimedia y funcionalidades adicionales (Timeline, Bio, Bloqueo, Búsqueda, Marketplace, Threads)](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/fase8.md)
9. [fase9.md — Integración del Servicio REST y Portal Web (FastAPI -> Go HTTP, Webhooks, Portal SPA)](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/fase9.md)
10. [fase10.md — Pruebas y optimización (Test Suite, Mocks, Concurrencia, Benchmarks)](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/fase10.md)
11. [fase11.md — Validación y liberación (Criterios de Aceptación, Transición y Despliegue)](file:///c:/Users/Administrator/Desktop/proyectos/fbreego/facebook-research/fbchat_v2/docs/migracion-go/fase11.md)

---

## 📊 Resumen Ejecutivo de la Auditoría

- **Repositorio analizado**: `facebook-research/fbchat_v2` (v2.3.3)
- **Total de archivos inspeccionados**: 63 archivos físicos (52 archivos `.py`, 1 plantilla web `.html`, 1 marcador `py.typed`, 9 documentos `.md`).
- **Capas funcionales identificadas**:
  1. `_core`: Gestión de sesión web (`dataGetHome`), parsing de tokens HTML (`fb_dtsg`, `jazoest`, `sessionID`, etc.), cliente HTTP centralizado, almacenamiento de cookies en disco/entorno, autenticación nativa Android FB4A con soporte 2FA TOTP y utilidades de formularios.
  2. `_features`: Acciones de usuario Facebook (publicar, archivar, borrar post, reaccionar a post, biografía, bloqueo, búsqueda, perfil profesional, marketplace) y administración de grupos Messenger (cambiar emoji, nombre de grupo, apodos, administradores, extracción de datos de threads).
  3. `_messaging`: Mensajería completa (envío de texto y adjuntos multipart, edición de mensajes, unsend, reacciones, notas efímeras de 24h, temas gráficos de chat, mensajes pendientes, escucha en tiempo real por MQTT y puente E2EE).
  4. `service*`: Microservicio REST y WebSocket construido con FastAPI, almacenamiento cifrado multi-tenant (AES-GCM / Fernet), despachador de webhooks con cola asíncrona, firma HMAC SHA-256 y protección SSRF, gestor de medios temporales y portal web SPA integrado (`portal.html`).
- **Estado de partida**: Toda la suite de migración comienza en estado **`PENDIENTE`**, sin modificación del código original y sin asumir capacidades no comprobadas.
