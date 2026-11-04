# Auditoría técnica de fbchatv2

**Fecha de corte:** 21 de octubre de 2026, America/La_Paz  
**Objeto:** defectos propios de `fbchat-v2` y del fork proveedor que consume SocialManager  
**Resultado:** 3 problemas confirmados vigentes, 2 problemas probables pendientes de validación, 0 riesgos críticos demostrados  
**Código de SocialManager:** no fue modificado durante esta auditoría

## 1. Resumen ejecutivo

SocialManager utiliza un servicio privado derivado de [`m008v/fbchat-v2`](https://github.com/m008v/fbchat-v2). La relación no es una inferencia basada sólo en nombres: el PR público cerrado y no fusionado [#34, `Feat/socialmanager container`](https://github.com/m008v/fbchat-v2/pull/34), fue creado desde el commit `3ac9e462c86ce295d88441dad4952de406a1352e`, parte del upstream `v2.3.2` y añadió el servicio FastAPI, sesiones, webhooks, portal y contenedor que describe la documentación local. SocialManager arranca ese checkout desde `SC_PROVIDERS/fbchat-v2` e instala `-e ...[service]` en [iniciar-socialmanager.bat](../../iniciar-socialmanager.bat#L39).

El checkout privado no está presente en esta copia: `SC_PROVIDERS/` está ignorado en [.gitignore](../../.gitignore#L23), no es submódulo y no hay un SHA fijado por SocialManager. Por ello se auditaron tres estados distinguibles:

1. el historial y las auditorías conservadas en SocialManager;
2. el último estado públicamente recuperable del fork, PR #34 (`3ac9e46`, 30 de septiembre de 2026);
3. el upstream publicado `v2.3.4`, commit de código `aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6`, del 21 de octubre de 2026.

Se confirmaron en `v2.3.4` tres defectos de librería: identidades HTTP internamente contradictorias y obsoletas; contratos privados de Facebook dispersos y hardcodeados, junto con extracción frágil de tokens desde HTML; y reintentos inmediatos sin una política común de backoff ni respeto de `Retry-After`. También quedan dos problemas probables: el flujo de login por credenciales basado en un perfil FB4A legado, respaldado por el issue abierto #30, y la pérdida de ediciones de Meta AI en el servicio del fork de SocialManager, cuya persistencia actual no se pudo comprobar sin `SC_PROVIDERS/`.

No existe evidencia suficiente para afirmar que uno de esos defectos haya causado por sí solo un bloqueo de cuenta. El issue histórico [#12](https://github.com/m008v/fbchat-v2/issues/12) comunica límite de conexiones y posterior prohibición de automatización, pero no contiene reproducción, telemetría, diagnóstico de Meta ni commit correctivo. Se utiliza únicamente como señal de riesgo histórica. En consecuencia, esta auditoría registra **cero causas críticas confirmadas de bloqueo**.

Quedaron fuera de los hallazgos principales los defectos exclusivos de SocialManager: deduplicación de destinos, materialización de tarjetas, concurrencia `P2002`, presentación de estados y consumo del evento `message.edited`. Se mencionan sólo cuando ayudan a demostrar qué emitió realmente el proveedor.

## 2. Alcance y metodología

### Alcance

- Historial Git completo disponible en este checkout de SocialManager, incluidos `git log`, `git blame`, `git diff` y contenido de commits.
- Auditorías de `docs/audits/`, su archivo histórico y la documentación de [fbchat-v2](../providers_chatgeneral/fbchat-v2.md).
- Arranque, instalación editable y puntos de integración del proveedor.
- Repositorio público `m008v/fbchat-v2`: código, tags, commits, changelog, issues, PRs, releases y CI.
- PR #34 como último ancestro público verificable del servicio usado por SocialManager.
- Autenticación, cookies, tokens, requests, headers, origins, referers, user agents, GraphQL, MQTT, E2EE, errores y reintentos.

### Método de clasificación

| Clase | Criterio aplicado |
|---|---|
| A. `fbchat-v2` | El defecto está en código, protocolo o mantenimiento de upstream o del repositorio proveedor, antes de que SocialManager consuma la respuesta. |
| B. SocialManager | El proveedor entrega un contrato correcto y el defecto nace sólo al adaptarlo, persistirlo o mostrarlo. Excluido de los hallazgos principales. |
| C. Compartido | Ambos lados contribuyen de forma necesaria. Se usa sólo como contexto y no como problema puro de librería. |
| D. Indeterminado | Faltan código, logs o una reproducción capaz de discriminar la causa. |

Un estado **confirmado** significa que el defecto de diseño o implementación sigue visible en el código auditado; no significa que se haya confirmado un bloqueo de cuenta. Un estado **probable** significa que existen código y reportes compatibles, pero falta una reproducción controlada o el checkout exacto desplegado.

### Comprobaciones ejecutadas

- Se verificaron hashes, tags, fechas, ramas y diferencias entre upstream y PR #34.
- Se inspeccionaron los commits locales que introdujeron o actualizaron cada auditoría. El historial de SocialManager sólo contiene siete commits y muchos documentos fueron importados en bloque; por ello la fecha escrita en un documento no se trató como fecha Git de introducción del defecto.
- Se contrastaron reportes antiguos con `v2.3.4`; los problemas corregidos no se declararon vigentes.
- Se consultó la API pública de GitHub para estados, fechas y cuerpos de issues/PRs.
- Se comprobó que la ejecución CI [`39624724769`](https://github.com/m008v/fbchat-v2/actions/runs/39624724769), sobre `b94a37d98871b30b5de958d7429357f72e5722b4`, finalizó correctamente. Ese HEAD sólo añade documentación sobre `v2.3.4`; el código publicado corresponde a `aa7abd0`.
- No se ejecutaron pruebas contra cuentas reales de Facebook: hacerlo habría requerido credenciales, producido tráfico externo y añadido riesgo innecesario. El entorno local tampoco tenía `pytest`; no se instalaron dependencias durante una auditoría de sólo lectura.

## 3. Versión y repositorio auditados

| Elemento | Identificación verificada | Alcance real |
|---|---|---|
| Upstream original | [`m008v/fbchat-v2`](https://github.com/m008v/fbchat-v2) | Proyecto que declara el paquete PyPI `fbchat-v2` y el namespace `fbchat_v2`. |
| Versión pública actual | [`v2.3.4`](https://github.com/m008v/fbchat-v2/releases/tag/v2.3.4), tag sobre `aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6` | Código principal usado para decidir si los defectos persisten. |
| HEAD público revisado | `b94a37d98871b30b5de958d7429357f72e5722b4` | Difiere del tag en documentación de memoria del agente, no en los módulos aquí citados. |
| Base del fork público | `981d0996db0a01af45526f8270d83559d5a3ef2e`, versión `2.3.2` | Base del PR #34. |
| Último commit público del fork | [`3ac9e462c86ce295d88441dad4952de406a1352e`](https://github.com/m008v/fbchat-v2/commit/3ac9e462c86ce295d88441dad4952de406a1352e) | Seis commits, 28 archivos, 6.441 adiciones; PR cerrado sin merge. |
| Checkout desplegable por SocialManager | `SC_PROVIDERS/fbchat-v2` | No presente, ignorado y sin SHA fijado. Su estado actual es indeterminado. |
| Contrato documentado localmente | [docs/providers_chatgeneral/fbchat-v2.md](../providers_chatgeneral/fbchat-v2.md) | REST `/api/v1`, WebSocket `/ws`, webhooks firmados y sesiones por cookies. Coincide con el servicio añadido por PR #34. |

La [documentación pública](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/README_EN.md#L5-L26) describe la librería como una integración **no oficial, basada en cuentas**. No debe confundirse con la Graph API oficial de Meta. SocialManager tampoco fija una versión: [iniciar-socialmanager.bat](../../iniciar-socialmanager.bat#L112) importa el checkout local y, si falta alguna dependencia, lo instala en modo editable. La afirmación correcta es, por tanto, “SocialManager usa un fork verificable de `m008v/fbchat-v2`”, no “SocialManager usa exactamente el wheel público 2.3.4”.

## 4. Historial de hallazgos desde SocialManager

| Fecha del incidente/auditoría | Evidencia desde SocialManager | Evolución y estado al corte | Clasificación |
|---|---|---|---|
| 30-09-2026 | PR público [#34](https://github.com/m008v/fbchat-v2/pull/34) añadió el servicio para SocialManager sobre `v2.3.2`. | Cerrado sin merge; el fork continuó fuera de upstream. Es el origen público verificable, no el estado desplegado actual. | A, procedencia |
| 09-10-2026 | [Auditoría de avatares](archive/10-09-2026_15-15_discard.md) encontró que `service_provider.py` descartaba `thumbSrc` y avatares LightSpeed. | Documentación posterior afirma propagación de `avatarUrl` y pruebas aprobadas. Auditoría `_discard`; no se presenta como vigente. | A, corregido en fork |
| 09-10-2026 | [Auditoría de tipo/nombre saliente](archive/10-09-2026_18-08_discard.md) registró tipo no autoritativo y nombres genéricos. | La documentación declara resolución por códigos directos y tipo efectivo. Auditoría `_discard`. | A, corregido en fork |
| 09/10-10-2026 | [Auditoría de historial](archive/10-09-2026_18-48_discard.md) mostró que el scraper de `facebook.com/messages/t/<id>` buscaba `lightspeed_web_request` en HTML Comet y devolvía cero mensajes. | Reemplazado en el fork por `MessengerGraphQLThreadFetcher`, `doc_id 3449967031715030`; luego se añadió validación de errores y registro configurable. Corregido, aunque confirma el riesgo estructural FBCV2-002. | A, corregido localmente |
| 10-10-2026 | [Auditoría de refresh](archive/10-10-2026_01-45_discard.md) encontró `fb_dtsg` obtenido de `m.facebook.com` y reutilizado en `www.facebook.com/api/graphqlbatch`, con `1357004/1357032`, seguido de falsa expiración y cierre de MQTT. | Marcada `_discard`; el código privado exacto de la corrección no está disponible. No se declara vigente. | A, corregido según registro |
| 11-10-2026 | [Auditoría de importación de cookies](archive/10-11-2026_09-30_discard.md) registró `WinError 32` por borrar SQLite antes de cerrar la conexión. | Se documenta cierre explícito y errores JSON; auditoría `_discard`. | A, corregido en fork |
| 11-10-2026 | [Auditoría de sesión expirada](10-11-2026_20-12_discard.md) registró `xs cookie was deleted`, falsa disponibilidad y fallo de un inbox SQLite antiguo sin `logical_key`. | Documentación actual del proveedor afirma clasificación 401, `session.status` y migración transaccional. Auditoría `_discard`. | A en proveedor; B contextual, corregidos |
| 12-10-2026 | [Auditoría de alias E2EE](10-12-2026_21-02_discard.md) probó que eventos salientes publicaban el `recipientId` como `threadId` en lugar del hilo técnico. | El proveedor documenta conservación `threadId -> recipientId/chatJid`; auditoría `_discard`. | A, corregido en fork |
| 12-10-2026 | [Auditoría Meta AI](10-12-2026_23-26.md) encontró que el bridge Go emitía `messageEdit`, el servicio Python lo filtraba y el código de hilo `201` caía como grupo. | SocialManager corrigió su mitad en `4803076`; el documento permanece abierto porque el proveedor privado no estaba disponible. Se conserva como FBCV2-005 probable. | A en proveedor, C en efecto total |
| 17-10-2026 | [Reauditoría](10-17-2026_22-59_discard.md) confirmó que `SC_PROVIDERS/` no existía en ese checkout. | La limitación persiste en el checkout actual. | D para persistencia del fork |
| 21-10-2026 | Upstream `v2.3.4`, commit [`2a25191`](https://github.com/m008v/fbchat-v2/commit/2a2519144bd3ee5921e624254852c962cfef1e35), incorporó supervisor único, backoff exponencial con jitter, clasificación de errores y limpieza de clientes MQTT/E2EE. | Mitiga el patrón histórico de conexiones del issue #12. El changelog reconoce que faltan pruebas live de ciertos reinicios y posibles gaps offline. | A, corregido upstream con límites |

### Lectura correcta del historial Git de SocialManager

- `5502018fc04dbd5fa874ce7b753a197c82a32579` (14-10-2026) importó gran parte de las auditorías y documentación canónica. No prueba que los defectos nacieran ese día.
- `48030760b67ab95cc069b89a027966daf834709d` (18-10-2026) añadió la nota que da por corregida la responsabilidad interna de Meta AI y deja pendiente el proveedor.
- Los commits anteriores del proveedor no forman parte del repositorio de SocialManager y no pudieron recuperarse desde su Git.
- Un commit de corrección o el sufijo `_discard` demuestra intención y estado documental, no por sí solo efectividad. Se aceptó “corregido” únicamente cuando el documento posterior describía el nuevo código y sus pruebas; aun así, sin el checkout privado no se volvió a ejecutar esa suite.

## 5. Problemas confirmados de fbchatv2

### FBCV2-001 — Identidad HTTP contradictoria y user agents obsoletos

1. **Identificador:** FBCV2-001.
2. **Título:** una misma sesión se presenta como navegadores, plataformas y aplicaciones incompatibles.
3. **Severidad:** alta.
4. **Estado:** confirmado en upstream `v2.3.4`; el impacto exacto sobre restricciones es sólo potencial.
5. **Descripción técnica:** el generador HTTP general rota aleatoriamente entre Windows, macOS y Linux, pero fija `sec-ch-ua-platform: "Windows"`. La carga del home usa Chrome 140/Windows con viewport 493; MQTT usa otra cadena Chrome 137; la subida de adjuntos rota agentes Chrome 20/22/42 y Safari 8; el login usa una aplicación Android/FB4A distinta. Además, el comentario del código declara que la rotación busca reducir detección por fingerprint. Ese objetivo produce incoherencia observable dentro de la misma sesión y no constituye una estrategia legítima de compatibilidad.
6. **Evidencia en el código de `fbchat-v2`:** [`_core/_utils.py` L38-L64](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_core/_utils.py#L38-L64), [`_core/_session.py` L47-L77](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_core/_session.py#L47-L77), [`_messaging/_attachments.py` L36-L44 y L111-L127](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_messaging/_attachments.py#L36-L44), y [`_messaging/_listening.py` L430-L480](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_messaging/_listening.py#L430-L480). El `git blame` atribuye el pool general y sus client hints al commit [`a90f940`](https://github.com/m008v/fbchat-v2/commit/a90f940).
7. **Commits relevantes de SocialManager:** ninguno demuestra la creación del defecto. `5502018` conserva auditorías de incompatibilidades de sesión y requests como contexto; PR #34 heredó la librería base. La integración no puede corregir estas cabeceras desde SocialManager.
8. **Issues, PRs o commits relacionados:** [issue #12](https://github.com/m008v/fbchat-v2/issues/12) reportó límite de conexiones y prohibición de automatización, pero no atribuye causalidad a los headers; [issue #27](https://github.com/m008v/fbchat-v2/issues/27) reportó checkpoint `1348023`. Se citan como señales, no como prueba. No se encontró PR que unifique el perfil HTTP.
9. **Impacto potencial:** respuestas HTML o checkpoints inesperados, fallos de upload, degradación de sesión y señales de integridad anómalas. La posibilidad de restricción existe por la naturaleza no oficial y la inconsistencia, pero no está demostrada por telemetría de Meta.
10. **Por qué pertenece a la librería:** los valores se construyen dentro de `_utils`, `_session`, `_attachments`, `_listening` y `_facebookLogin` antes de que SocialManager participe. Clasificación A.
11. **Solución recomendada:** eliminar la rotación “anti-fingerprint”; usar un perfil de cliente único, coherente y conservador por sesión; generar `sec-ch-*` sólo cuando correspondan al user agent real; evitar headers de navegador innecesarios; compartir un constructor tipado para home, GraphQL, MQTT y upload; añadir pruebas que impidan contradicciones de plataforma y versión. Esto mejora compatibilidad y cumplimiento, no busca evadir controles.
12. **Fuentes verificables:** enlaces de código anteriores, [repositorio](https://github.com/m008v/fbchat-v2), [issue #12](https://github.com/m008v/fbchat-v2/issues/12), [issue #27](https://github.com/m008v/fbchat-v2/issues/27).

### FBCV2-002 — Contratos privados hardcodeados y extracción frágil de sesión

1. **Identificador:** FBCV2-002.
2. **Título:** endpoints, `doc_id` y tokens internos de Facebook carecen de un registro versionado y validación robusta.
3. **Severidad:** alta para disponibilidad y autenticación; baja como causa directa de bloqueo.
4. **Estado:** confirmado en upstream `v2.3.4`.
5. **Descripción técnica:** la librería depende de endpoints no documentados (`/api/graphql/`, `/api/graphqlbatch/`, `/ajax/mercury/upload.php`), IDs de operaciones repartidos por módulos y marcadores literales del HTML del home. `_session.py` extrae `fb_dtsg`, `sessionId`, `actorID` y `client_revision` con cortes de cadenas exactos. Cuando Meta cambia markup, operation IDs o esquemas, el fallo puede ser total y difícil de distinguir de una sesión expirada.
6. **Evidencia en el código de `fbchat-v2`:** [`_core/_session.py` L82-L118](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_core/_session.py#L82-L118) contiene los marcadores exactos y retorna `None`; [`_features/_thread/_all_thread_data.py` L37-L85](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_features/_thread/_all_thread_data.py#L37-L85) fija `graphqlbatch` y `doc_id 3336396659757871`; [`_messaging/_changeTheme.py` L44-L48](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_messaging/_changeTheme.py#L44-L48) fija query y versión de tema; [`_messaging/_attachments.py` L36-L44](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_messaging/_attachments.py#L36-L44) fija el upload Mercury legado.
7. **Commits relevantes de SocialManager:** `5502018` incorporó la [auditoría del scraper HTML](archive/10-09-2026_18-48_discard.md) y la documentación de su reemplazo. El fork añadió un `GraphQLRegistry` configurable para `ThreadMessagesQuery`, pero esa mejora no existe de forma general en upstream `v2.3.4` y no cubre todos los IDs.
8. **Issues, PRs o commits relacionados:** [#20](https://github.com/m008v/fbchat-v2/issues/20) y [#21](https://github.com/m008v/fbchat-v2/issues/21) documentaron que un endpoint devolvía HTML y el parser terminaba en `IndexError`; fueron cerrados años después y el parser actual de envío ya falla de forma estructurada. La recurrencia histórica y el scraper roto de SocialManager demuestran fragilidad, no que esos issues sigan abiertos.
9. **Impacto potencial:** login que retorna `None`, historiales vacíos, operaciones que dejan de funcionar tras un cambio de Meta, falsos diagnósticos de expiración y caídas parciales difíciles de observar. No hay evidencia de que un `doc_id` obsoleto por sí solo cause restricciones de cuenta.
10. **Por qué pertenece a la librería:** los contratos privados, parsers y IDs están codificados en el paquete. SocialManager sólo recibe éxito, vacío o error. Clasificación A.
11. **Solución recomendada:** crear un registro central tipado de operaciones con friendly name, ID, esquema esperado, fecha/versión y override controlado; validar content type y estructura antes de parsear; distinguir explícitamente “markup/protocolo cambió” de “sesión expirada”; añadir canaries de sólo lectura y fixtures sanitizados; retirar endpoints legacy cuando exista una ruta estable. No descargar ni descubrir IDs de forma encubierta en producción.
12. **Fuentes verificables:** código enlazado, [issues #20](https://github.com/m008v/fbchat-v2/issues/20) y [#21](https://github.com/m008v/fbchat-v2/issues/21), [auditoría local de historial](archive/10-09-2026_18-48_discard.md), [documentación local L106-L117](../providers_chatgeneral/fbchat-v2.md#L106).

### FBCV2-003 — Reintentos inmediatos y no diferenciados en rutas GraphQL

1. **Identificador:** FBCV2-003.
2. **Título:** ciertas consultas repiten requests sin backoff, jitter ni `Retry-After`.
3. **Severidad:** media.
4. **Estado:** confirmado en upstream `v2.3.4`.
5. **Descripción técnica:** el listado de threads intenta hasta tres veces y la variante async sólo hace `sleep(0)`. El módulo de temas también usa tres intentos por defecto y captura cualquier `httpx.HTTPError`; eso incluye respuestas 4xx/5xx elevadas por `raise_for_status`, sin discriminar autenticación, rate limit o error transitorio. La política robusta añadida en `v2.3.4` aplica a reconexión MQTT/E2EE, no a estos requests GraphQL.
6. **Evidencia en el código de `fbchat-v2`:** [`_all_thread_data.py` L37-L39](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_features/_thread/_all_thread_data.py#L37-L39) fija dos reintentos y [L94-L127](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_features/_thread/_all_thread_data.py#L94-L127) los ejecuta sin espera; [`_changeTheme.py` L44-L48](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_messaging/_changeTheme.py#L44-L48) fija dos reintentos y [L102-L186](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_messaging/_changeTheme.py#L102-L186) repite timeouts y cualquier `HTTPError` inmediatamente.
7. **Commits relevantes de SocialManager:** no se encontró un commit de SocialManager que origine este comportamiento. Las auditorías de errores GraphQL importadas por `5502018` motivaron fail-closed en el fork, pero no una política global de rate limit.
8. **Issues, PRs o commits relacionados:** [`2a25191`](https://github.com/m008v/fbchat-v2/commit/2a2519144bd3ee5921e624254852c962cfef1e35) es una referencia positiva: implementa backoff exponencial con jitter y clasificación para MQTT/E2EE, pero no modifica los módulos citados. No se encontró issue específico para estos reintentos HTTP.
9. **Impacto potencial:** ráfagas de solicitudes durante timeouts, repetición inútil ante 401/403/429, mayor latencia y presión adicional durante degradaciones. El volumen es acotado —tres intentos por llamada—, por lo que no se clasifica como crítico ni como causa confirmada de restricción.
10. **Por qué pertenece a la librería:** los loops, excepciones y tiempos están implementados dentro de los módulos de `fbchat-v2`; SocialManager realiza una operación lógica y no controla cada intento. Clasificación A.
11. **Solución recomendada:** reutilizar una política HTTP central: retry sólo para fallos transitorios y métodos/operaciones idempotentes, backoff exponencial con jitter, límite total por operación, respeto de `Retry-After`, circuit breaker por sesión y cero retry automático para autenticación, checkpoint, 4xx permanentes o mutaciones de resultado incierto.
12. **Fuentes verificables:** código enlazado, [commit de reconnect `2a25191`](https://github.com/m008v/fbchat-v2/commit/2a2519144bd3ee5921e624254852c962cfef1e35), [changelog 2.3.4](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/CHANGELOG.md#L46-L75).

## 6. Problemas probables pendientes de validación

### FBCV2-004 — Login por credenciales atado a un perfil FB4A legado

1. **Identificador:** FBCV2-004.
2. **Título:** el login simula una versión antigua y regionalizada de Facebook para Android.
3. **Severidad:** alta para autenticación.
4. **Estado:** probable; código confirmado, causalidad del rechazo pendiente de una reproducción controlada.
5. **Descripción técnica:** el login llama `b-graph.facebook.com/auth/login` con API key y app token por defecto, user agent FB4A 340 sobre Android 7.1.2, locale/país `vi_VN`/`VN`, operador Viettel y dispositivo Samsung, mientras genera IDs de dispositivo nuevos por instancia. El issue abierto #30 informa credenciales y 2FA correctos rechazados con `401/1348131`. No hay evidencia pública que aísle qué campo causa el rechazo.
6. **Evidencia en el código de `fbchat-v2`:** [`_core/_facebookLogin.py` L30-L42](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_core/_facebookLogin.py#L30-L42) define endpoint, credenciales de aplicación y perfil; [L251-L324](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/src/fbchat_v2/_core/_facebookLogin.py#L251-L324) genera device IDs y fija locale/país. Los overrides por entorno para key/token reducen rigidez, pero no corrigen el perfil restante.
7. **Commits relevantes de SocialManager:** no hay commit que cause el rechazo. La UI sólo entrega las credenciales al proveedor. La auditoría local de sesiones importada en `5502018` muestra que SocialManager depende de la clasificación correcta del error remoto.
8. **Issues, PRs o commits relacionados:** [#30](https://github.com/m008v/fbchat-v2/issues/30) sigue abierto desde el 25-05-2026; [#27](https://github.com/m008v/fbchat-v2/issues/27) documentó checkpoint `1348023`. El comentario del mantenedor en #30 propuso proxies, pero no presentó medición ni corrección y no se adopta aquí como recomendación.
9. **Impacto potencial:** falsos “credenciales inválidas”, checkpoints, imposibilidad de vincular algunas cuentas y señales de dispositivo inconsistentes. Puede contribuir a revisiones de seguridad, pero no se ha demostrado que produzca bloqueos.
10. **Por qué pertenece a la librería:** SocialManager no construye el request de `b-graph`; todos los valores observados y la interpretación del resultado pertenecen a `_facebookLogin.py`. Clasificación A si se valida; por ahora la causa concreta es D.
11. **Solución recomendada:** tratar el login por contraseña como capacidad experimental; preferir una sesión obtenida por un flujo autorizado del usuario; conservar y exponer códigos/subcódigos sin traducirlos a contraseña inválida; no reintentar checkpoints; retirar constantes regionales que no reflejan el entorno; persistir sólo identificadores legítimos de la sesión cuando sea necesario; documentar que el usuario debe completar verificaciones directamente en Meta. No usar rotación de proxies para eludir controles.
12. **Fuentes verificables:** código enlazado, [issue #30](https://github.com/m008v/fbchat-v2/issues/30), [comentario del mantenedor](https://github.com/m008v/fbchat-v2/issues/30#issuecomment-4583551958), [issue #27](https://github.com/m008v/fbchat-v2/issues/27).

### FBCV2-005 — El servicio del fork pierde ediciones de Meta AI y clasifica `201` como grupo

1. **Identificador:** FBCV2-005.
2. **Título:** ruptura del pipeline `LSEditMessage -> messageEdit -> webhook` en el fork proveedor.
3. **Severidad:** alta para integridad de mensajes.
4. **Estado:** probable/pending en el fork actual; confirmado en el estado auditado el 12-10-2026 y compatible con el último ancestro público.
5. **Descripción técnica:** Meta AI puede insertar primero un mensaje vacío y completar su texto mediante `LSEditMessage`. El bridge Go ya modelaba `messageEdit`, pero el servicio Python del proveedor no lo admitía en su cola ni lo convertía a webhook. Además, el código de thread `AI_BOT = 201` no estaba en el conjunto directo y caía al fallback de grupo, contaminando la caché de tipos. El efecto era mensaje permanente vacío y dos conversaciones lógicas.
6. **Evidencia en el código de `fbchat-v2`:** en el commit público del fork, [`bridge-e2ee/bridge/events.go` L47 y L346](https://github.com/m008v/fbchat-v2/blob/3ac9e462c86ce295d88441dad4952de406a1352e/bridge-e2ee/bridge/events.go#L47) define y emite `messageEdit`, mientras una búsqueda completa de `src/fbchat_v2/service*.py` en ese commit no contiene `messageEdit`. La [auditoría local L22-L31](10-12-2026_23-26.md#L22) registra las líneas posteriores del fork privado (`service.py:229-236` y `1259`) y evidencia operativa.
7. **Commits relevantes de SocialManager:** `5502018` incorporó la auditoría; `4803076` añadió que SocialManager ya soporta `message.edited` y fuerza directos cuando `conversationId === actorId`, dejando pendiente el proveedor. Esos cambios corrigen el consumidor, no el origen.
8. **Issues, PRs o commits relacionados:** [PR #34](https://github.com/m008v/fbchat-v2/pull/34), commit [`3ac9e46`](https://github.com/m008v/fbchat-v2/commit/3ac9e462c86ce295d88441dad4952de406a1352e). No se encontró issue upstream dedicado; el servicio FastAPI nunca se fusionó y ya no está en upstream `v2.3.4`.
9. **Impacto potencial:** pérdida silenciosa de contenido, historial incompleto, rutas duplicadas y respuestas de Meta AI irrecuperables desde el webhook inicial. No es un vector demostrado de bloqueo de cuenta.
10. **Por qué pertenece a la librería:** la primera pérdida sucede dentro del repositorio proveedor, entre el bridge Go y el servicio Python, antes de enviar el webhook. La incapacidad histórica de SocialManager para consumir ediciones era un defecto B separado y ya fue corregido. Para el defecto de emisión, clasificación A del fork; para la bifurcación total histórica, C.
11. **Solución recomendada:** admitir `messageEdit` en la cola, convertirlo a `message.edited` con `messageId`, texto y thread canónicos, clasificar explícitamente `201` como directo/AI, no persistir el fallback de grupo sin evidencia de membresía y añadir pruebas end-to-end insert-vacío/edición. Verificar en `SC_PROVIDERS/fbchat-v2` antes de cerrar la auditoría y conservarla abierta si el código sigue ausente.
12. **Fuentes verificables:** [auditoría principal](10-12-2026_23-26.md), [reauditoría de ausencia del proveedor](10-17-2026_22-59_discard.md), [PR #34](https://github.com/m008v/fbchat-v2/pull/34), [bridge en `3ac9e46`](https://github.com/m008v/fbchat-v2/blob/3ac9e462c86ce295d88441dad4952de406a1352e/bridge-e2ee/bridge/events.go#L340-L360).

## 7. Riesgos de autenticación y restricciones de cuentas

### Lo que sí está demostrado

- La librería usa APIs privadas/no oficiales con cookies o credenciales de una cuenta personal.
- Los fingerprints de request son contradictorios (FBCV2-001).
- Algunas cuentas fallan en el login por credenciales con `401/1348131` y el issue sigue abierto (FBCV2-004).
- Hubo un reporte histórico de máximo de 50 conexiones seguido de prohibición de automatización ([#12](https://github.com/m008v/fbchat-v2/issues/12)).
- Hubo reportes de security checkpoint `1348023` ([#27](https://github.com/m008v/fbchat-v2/issues/27)).
- `v2.3.4` corrigió una debilidad real de reconexión: ahora un supervisor único aplica backoff con jitter y detiene errores de auth/checkpoint/protocolo.

### Lo que no está demostrado

- No hay respuesta de Meta, experimento A/B, captura de tráfico antes/después ni reproducción que atribuya un bloqueo a un header, endpoint o ritmo concreto.
- El issue #12 no demuestra que `fbchat-v2` abriera realmente 50 sockets; tampoco vincula el cierre a un commit.
- Un checkpoint o `xs` eliminado puede ser una decisión de seguridad de Meta, expiración legítima o incompatibilidad; no equivale por sí solo a ban.
- No se verificó el comportamiento live del fork privado actual.

### Evaluación

| Riesgo | Evidencia | Nivel | Confianza |
|---|---|---:|---:|
| Restricción por naturaleza no oficial/account-based | Declaración del proyecto y uso de endpoints privados | alta | alta |
| Señales anómalas por fingerprint contradictorio | Código actual | alta | media para el efecto, alta para la inconsistencia |
| Checkpoint/rechazo por login legado | Código + issues #27/#30 | alta | media-baja para causa concreta |
| Presión por retries HTTP | Código actual, máximo tres intentos por llamada | media | media |
| Tormenta de reconnect MQTT/E2EE | Histórica; mitigada en 2.3.4 | baja residual | media |
| Causa crítica confirmada de bloqueo | Ninguna | **0** | alta |

Las correcciones deben reducir tráfico, hacer los requests coherentes y detenerse ante señales de autenticación. No se recomienda alterar proxies, fingerprints o ritmos con el fin de evadir detección.

## 8. Headers, origins y valores hardcodeados

| Ruta/flujo | Valores observados en `v2.3.4` | Problema |
|---|---|---|
| HTTP general | UA aleatorio Chrome 137 en Windows/macOS/Linux; `sec-ch-ua-platform` siempre Windows; `Origin` y `Referer` derivados sólo del host | Contradicción interna y referer sin ruta contextual. |
| Home/session | Chrome 140, Windows, platform-version 19.0.0, idiomas VI/FR/EN, viewport 493 | Perfil fijo diferente al general y a MQTT. |
| Adjuntos | Chrome 20/22/42, Safari 8, Windows 6.1/6.3, OS X 10.10; `upload.facebook.com/ajax/mercury/upload.php` | Agentes muy antiguos y endpoint Mercury legado. |
| MQTT | app id `219994525426954`, región `eag`, client id `mqttwsclient`, Chrome 137 abreviado | Valores privados fijos y otro perfil de navegador. |
| Login FB4A | API key/app token por defecto, FBAV 340, Android 7.1.2, Samsung, Viettel, `vi_VN`, `VN` | Perfil regional/legado ajeno al contexto real de muchas cuentas. |
| Threads | `/api/graphqlbatch/`, `doc_id 3336396659757871` | Operación privada sin registry general ni versión. |
| Temas | `doc_id 24474714052117636`, `THEME_VERSION_ID 24631415369801570` | Dos identificadores privados fijados en módulo. |
| Tokens de sesión | Literales `DTSGInitialData`, `async_get_token`, `sessionId`, `actorID`, `client_revision` | Dependencia de serialización HTML interna. |

La existencia de constantes privadas es inherente a una integración no oficial, pero dispersarlas, combinarlas con perfiles incompatibles y no distinguir drift de expiración constituye el defecto mantenible.

## 9. Dependencias y compatibilidad

### Dependencias públicas

`v2.3.4` declara Python `>=3.10` y dependencias con sólo límite inferior: `httpx>=0.27.0`, `requests>=2.32.0`, `paho-mqtt>=1.6.1` y `pyotp>=2.9.0`. No se confirmó una CVE vigente en esas dependencias a partir de la evidencia disponible. Los rangos sin límite superior sí amplían la matriz de comportamiento posible y deberían cubrirse con resolución lockeada en despliegues.

El CI público del 21-10-2026 terminó en éxito y cubre Python 3.10–3.14, Windows, wheel/sdist, pruebas Go, race detector, `govulncheck` y RPC del bridge en varios sistemas. Es evidencia positiva de calidad, pero no valida endpoints de Facebook ni cuentas reales.

### Bridge E2EE

El issue [#35](https://github.com/m008v/fbchat-v2/issues/35) señaló correctamente que un hash embebido en el mismo paquete no es una fuente de confianza independiente. En el estado actual existen versión y SHA fijados, hosts/redirects restringidos, límite de tamaño, `SHA256SUMS` y attestations de release. Por tanto, no se eleva como hallazgo vigente independiente; queda como riesgo residual de cadena de suministro y reproducibilidad.

### Compatibilidad y ramas

- El fork de SocialManager nació sobre `v2.3.2` y no fue fusionado. No hay evidencia de que haya incorporado `v2.3.3` o `v2.3.4`.
- El script local aún referencia un toolchain Go bajo `.tmp/go1.26.6`, mientras upstream `v2.3.4` elevó su build a Go `1.26.10`. Sin el checkout no se puede saber si el binario ya fue actualizado; se registra como discrepancia a verificar, no como defecto confirmado.
- Upstream `v2.3.4` declara explícitamente que el cold restart tras payload real y la pérdida aislada del bridge requieren validación live; tampoco garantiza recuperar todos los mensajes durante una caída.
- El servicio FastAPI de PR #34 no existe en upstream actual. Actualizar ciegamente a `v2.3.4` no preservaría el contrato de SocialManager; requiere rebase y pruebas del fork.

### Higiene histórica de secretos

El PR fusionado [#31](https://github.com/m008v/fbchat-v2/pull/31) corrigió una cookie real rastreada en `src/config.json` y añadió el archivo a `.gitignore` mediante `ff72f482af9c27b3d4ff945697a05764ec287d9e`. Fue una exposición grave histórica de la librería, pero está corregida en el árbol actual. La remediación completa de una cookie expuesta también exige revocación/rotación; Git no permite comprobar si el titular la realizó.

## 10. Matriz de severidad y prioridades

| ID | Hallazgo | Propietario | Severidad | Estado | Prioridad |
|---|---|---|---:|---|---:|
| FBCV2-001 | Identidad HTTP contradictoria/obsoleta | Upstream | alta | confirmado | P0 |
| FBCV2-002 | Contratos privados hardcodeados y parser frágil | Upstream | alta | confirmado | P0 |
| FBCV2-003 | Reintentos GraphQL inmediatos/no diferenciados | Upstream | media | confirmado | P1 |
| FBCV2-004 | Login FB4A legado y regionalizado | Upstream | alta | probable | P0 |
| FBCV2-005 | Meta AI: edición perdida y código 201 como grupo | Fork proveedor | alta | probable/pending | P0 para SocialManager |

**Conteos:** 3 confirmados; 2 probables; 4 de severidad alta; 1 media; 0 críticos. “P0” indica orden de corrección/validación, no severidad crítica.

## 11. Recomendaciones de corrección en fbchatv2

1. **Unificar el transporte HTTP (P0).** Introducir un `SessionClientProfile` único, inmutable por sesión, usado por home, GraphQL, upload y MQTT. Eliminar la rotación “anti-fingerprint” y los UA Chrome 20–42.
2. **Aislar el login legado (P0).** Marcarlo experimental, preservar códigos de Meta, detener retries ante checkpoint y priorizar reautenticación explícita del usuario. No proponer proxies como bypass.
3. **Centralizar contratos privados (P0).** Registry tipado de endpoint, operation/friendly name, `doc_id`, esquema de respuesta y override auditado. Aplicarlo a todas las operaciones, no sólo a mensajes de thread en el fork.
4. **Corregir Meta AI en el fork (P0).** Completar `messageEdit -> message.edited`, mapear tipo `201`, añadir regresión end-to-end y validar el checkout real antes de cerrar la auditoría de 12-10.
5. **Aplicar una política HTTP común (P1).** Backoff/jitter, `Retry-After`, presupuesto total, clasificación de errores y cero reenvío automático de mutaciones inciertas.
6. **Mejorar diagnóstico de sesión (P1).** Separar al menos `auth_expired`, `checkpoint_required`, `protocol_drift`, `operation_id_stale`, `rate_limited`, `invalid_content_type` y `network_transient`; nunca convertir todos en “credenciales inválidas”.
7. **Fijar procedencia del fork (P1).** Registrar remote, branch, SHA upstream/base y SHA desplegado; publicar un changelog del fork y rebasarlo explícitamente sobre 2.3.4. SocialManager debería consumir un commit o artefacto inmutable, no sólo un checkout editable.
8. **Pruebas seguras (P1).** Fixtures sanitizados para HTML/GraphQL, property tests de headers, pruebas de rate limit simuladas, tests de expiración `xs`, y canary opt-in con cuenta de prueba autorizada. Nunca usar cuentas de producción para descubrir comportamiento.
9. **Cadena de suministro (P2).** Mantener attestations/SHA, documentar verificación externa y ofrecer modo obligatorio de binario local. Rotar inmediatamente cualquier cookie que llegue a Git.

## 12. Evidencias y referencias de GitHub

### Repositorio, versión y procedencia

- [Repositorio upstream `m008v/fbchat-v2`](https://github.com/m008v/fbchat-v2)
- [Release `v2.3.4`](https://github.com/m008v/fbchat-v2/releases/tag/v2.3.4)
- [Commit de release `aa7abd0`](https://github.com/m008v/fbchat-v2/commit/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6)
- [Changelog](https://github.com/m008v/fbchat-v2/blob/aa7abd0386ef1d4cabf8ce592d0f2cf308d8f4f6/CHANGELOG.md)
- [CI exitosa sobre HEAD](https://github.com/m008v/fbchat-v2/actions/runs/39624724769)
- [PR #34: fork de servicio SocialManager](https://github.com/m008v/fbchat-v2/pull/34)
- [Último commit del PR #34](https://github.com/m008v/fbchat-v2/commit/3ac9e462c86ce295d88441dad4952de406a1352e)

### Issues y PRs relevantes

- [#12: máximo de conexiones y reporte de prohibición de automatización](https://github.com/m008v/fbchat-v2/issues/12) — señal histórica, sin causa raíz.
- [#20](https://github.com/m008v/fbchat-v2/issues/20) y [#21](https://github.com/m008v/fbchat-v2/issues/21) — respuesta HTML y parser de envío roto; cerrados, parser actual endurecido.
- [#22: deltas NoOp](https://github.com/m008v/fbchat-v2/issues/22) — cerrado con “fix done”, sin evidencia suficiente para elevarlo como vigente.
- [#27: checkpoint `1348023`](https://github.com/m008v/fbchat-v2/issues/27) — cerrado; no hubo corrección técnica trazable en el hilo.
- [#30: credenciales correctas rechazadas con `1348131`](https://github.com/m008v/fbchat-v2/issues/30) — abierto.
- [#35: verificación del bridge E2EE](https://github.com/m008v/fbchat-v2/issues/35) — cerrado como no planificado; mitigaciones actuales revisadas.
- [PR #31: cookie real rastreada y saneada](https://github.com/m008v/fbchat-v2/pull/31) — corregido históricamente.
- [Commit `2a25191`: reconexión segura](https://github.com/m008v/fbchat-v2/commit/2a2519144bd3ee5921e624254852c962cfef1e35) — mitigación de retries MQTT/E2EE.

### Evidencia local de SocialManager

- [Documentación del proveedor](../providers_chatgeneral/fbchat-v2.md)
- [Auditoría Meta AI abierta](10-12-2026_23-26.md)
- [Reauditoría del proveedor ausente](10-17-2026_22-59_discard.md)
- [Auditoría de expiración `xs`](10-11-2026_20-12_discard.md)
- [Auditoría de alias E2EE](10-12-2026_21-02_discard.md)
- [Auditoría histórica del scraper](archive/10-09-2026_18-48_discard.md)
- [Auditoría histórica de refresh/token](archive/10-10-2026_01-45_discard.md)
- [Auditoría histórica de importación SQLite](archive/10-11-2026_09-30_discard.md)

## 13. Limitaciones de la auditoría

1. `SC_PROVIDERS/fbchat-v2` no existe en el checkout, está ignorado y tampoco existe la auditoría delegada enlazada mediante una ruta antigua `D:`. No se creó un repositorio sustituto ni se inventó su contenido.
2. No hay lock, submódulo o SHA que identifique la versión desplegada del fork. PR #34 sólo demuestra procedencia y un estado público anterior.
3. El historial Git local de SocialManager empieza con imports grandes del 14/17 de octubre; no conserva los commits originales del proveedor ni todas las fechas de implantación.
4. Los documentos `_discard` describen correcciones, pero sus suites no pudieron repetirse sin el proveedor privado. Se contrastaron con documentación posterior y se excluyeron como vigentes.
5. No se usaron credenciales, cookies ni cuentas reales. Por ello no se validaron endpoints live, checkpoints, bans, rate limits ni drift geográfico.
6. La suite upstream no se ejecutó localmente porque `pytest` no estaba instalado. Se verificó la ejecución CI pública exitosa y se inspeccionó el código estático.
7. Dependabot/security alerts no son públicos mediante la API sin permisos; no se puede afirmar que “no existen alertas”. Sí se verificó que CI incluye `govulncheck` y concluyó correctamente.
8. GitHub y Meta son estados externos cambiantes. Los enlaces y estados reflejan el corte del 21-10-2026.
9. Los issues de terceros contienen observaciones, no diagnósticos forenses. Ninguno se promovió por sí solo a causa raíz.

Según la regla de delegación del proyecto, los defectos externos deberían copiarse a `SC_PROVIDERS/audits/`. No fue posible hacerlo porque ese repositorio no está disponible en este workspace; crear el directorio ignorado habría simulado una delegación sin propietario ni historial. Este informe identifica explícitamente como responsables a upstream (`m008v/fbchat-v2`) para FBCV2-001 a 004 y al mantenedor del fork proveedor para FBCV2-005.

## 14. Conclusiones

La evidencia permite atribuir a `fbchat-v2`, y no exclusivamente a SocialManager, tres problemas actuales: requests con identidades incompatibles, dependencia privada hardcodeada y frágil, y retries HTTP sin política robusta. Existen además dos problemas probables que requieren validación focalizada: el login FB4A legado y la pérdida de ediciones de Meta AI en el fork proveedor.

No se encontró una causa crítica demostrada de bloqueo de cuentas. Sí existe una combinación de riesgo alta —API no oficial, perfiles inconsistentes, login legado y algunos retries— que justifica reducir superficie y volumen, mejorar la clasificación de errores y detenerse ante checkpoints. La respuesta técnica correcta es robustez y transparencia, no evasión.

La mayoría de los incidentes históricos detectados por SocialManager —scraping HTML, mezcla de tokens, SQLite en Windows, expiración `xs`, dirección/alias E2EE, avatares y tipos efectivos— aparecen documentados como corregidos y no se contaron como hallazgos vigentes. El único incidente local que debe seguir abierto es Meta AI hasta inspeccionar el checkout real de `SC_PROVIDERS/fbchat-v2`.

**Resumen final:**

- Problemas confirmados vigentes: **3**.
- Problemas probables pendientes: **2**.
- Riesgos críticos demostrados: **0**.
- Hallazgos de severidad alta: **4**.
- Correcciones prioritarias: perfil HTTP único; aislamiento del login legado; registry de contratos GraphQL; soporte completo de `messageEdit`/tipo `201`; política central de retries.
