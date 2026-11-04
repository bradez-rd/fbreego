# Resumen global de auditoría frente a `facebook-research`

**Fecha de corte:** 2026-10-21  
**Estado:** completo

## Inventario y progreso

El descubrimiento de límites Git identificó un único repositorio de proyecto actual. Los repositorios bajo `facebook-research/` son material de referencia y no forman parte de la cola de repositorios actuales.

| Repositorio actual | Estado | Informe | Hallazgos confirmados | Hipótesis |
|---|---|---|---:|---:|
| `fbreego` | Analizado y verificado | [`fbreego-audit.md`](./fbreego-audit.md) | 18 | 2 |

Repositorios actuales pendientes: **ninguno**.

## Referencias comparadas

| Referencia | Área principal utilizada |
|---|---|
| `facebook-research/fbchat_v2` | Contrato del servicio, listeners, eventos, sesiones y mensajería |
| `facebook-research/mautrix-meta` | Transporte DGW/LightSpeed, sincronización, resiliencia y E2EE en Go |
| `facebook-research/fb-messenger-e2ee` | Noise, Signal, device store, prekeys y media E2EE |
| `facebook-research/fca-unofficial` | Inventario de acciones y refresco de sesión |
| `facebook-research/facebook-marketplace-mcp` | Búsqueda, ubicación, monitores y rate limiting de Marketplace |

## Oportunidades compartidas y dependencias

```text
Seguridad y contratos
├── autenticación fail-closed
├── secreto obligatorio y aislamiento tenant
└── límites/redacción de tráfico
        │
        v
Cliente HTTP resiliente
├── clasificación de errores
├── refresh de tokens
├── retry/rate limit
└── catálogo de operaciones GraphQL
        │
        v
Transporte Messenger real
├── conexión y reconexión
├── cursores/seq persistentes
└── eventos normalizados
        │
        ├──> WebSocket + webhooks
        └──> E2EE Noise/Signal
                  │
                  v
        Features adicionales y Marketplace
```

Las mejoras de seguridad y contrato son prerrequisitos para ampliar la superficie pública. El cliente HTTP resiliente debe preceder nuevas operaciones GraphQL. La recepción real debe existir antes de declarar funcionales WebSocket, webhooks o E2EE. Los monitores de Marketplace dependen además de persistencia y entrega de eventos fiables.

## Prioridades globales

1. Corregir autenticación, claves, tenant paths, inspector de tráfico y endpoint productivo de tests.
2. Dejar de anunciar listener/E2EE como disponibles mientras sean fachadas.
3. Corregir envío de adjuntos y conectar el bus de eventos con webhooks.
4. Centralizar HTTP, refresh de tokens, retry, rate limit y errores tipados.
5. Implementar transporte real y sincronización persistente.
6. Integrar E2EE con revisión de licencias y pruebas criptográficas.
7. Ampliar funciones Messenger/Marketplace de forma incremental.
8. Sincronizar router, OpenAPI, README y matriz de capacidades.

## Riesgos transversales

- Meta usa APIs privadas y operaciones GraphQL que pueden cambiar sin aviso.
- La automatización puede causar checkpoints o bloqueo de cuentas.
- `mautrix-meta` y `fb-messenger-e2ee` son AGPL-3.0; no deben copiarse o vincularse sin revisión legal.
- Cambiar el cifrado de sesiones necesita formato versionado y migración.
- Una cola durable introduce semántica at-least-once e idempotencia.
- Las pruebas live deben usar cuentas dedicadas y ser opt-in.

## Criterio de cierre

- Informe individual existente y no vacío: **sí**.
- Doce secciones obligatorias: **sí**.
- Evidencias con rutas concretas: **sí**.
- Hallazgos confirmados separados de hipótesis: **sí**.
- Roadmap priorizado y dependencias: **sí**.
- Código de producción o dependencias modificados: **no**.

