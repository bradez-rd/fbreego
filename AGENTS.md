# AGENTS.md — Guía para agentes que mantienen la librería no oficial de Messenger

> **Alcance:** esta librería implementa acceso **no oficial** a Facebook Messenger para cuentas personales. **No** se basa en Messenger Platform / Graph API para páginas y bots. Los protocolos privados pueden cambiar sin aviso. Este archivo orienta al agente; el código, las pruebas y la documentación **realmente presentes** en el repositorio son la fuente de verdad sobre su arquitectura.

## 1. Objetivo y prioridades

Al corregir un error, modificar una implementación o añadir una función:

1. **Entender** el comportamiento actual y reproducir el problema o definir el contrato de la función.
2. **Investigar** primero el protocolo y el estado de las implementaciones relacionadas, cuando corresponda.
3. **Diseñar un cambio mínimo**, compatible con la API pública cuando sea posible.
4. **Implementar con pruebas**, sin exponer credenciales ni sesiones.
5. **Documentar** las decisiones, la evidencia y cualquier incompatibilidad.

Priorizar, en este orden: seguridad de cuentas y datos; corrección del protocolo; estabilidad; compatibilidad; rendimiento; ergonomía de API. No sacrificar seguridad por mantener una función operativa.

## 2. Reconocimiento obligatorio del repositorio

Antes de editar:

- Leer README, documentación, manifiesto de dependencias, historial reciente, tests y archivos de configuración. Buscar otros archivos `AGENTS.md` más específicos y seguir sus instrucciones aplicables.
- Identificar lenguaje, gestor de paquetes, comandos reales de lint/build/tests, API pública y módulos de autenticación, transporte, parsing, sincronización y cifrado.
- Localizar los puntos de entrada y consumidores del comportamiento que se va a modificar. Buscar tests y reportes de bugs relacionados.
- **No asumir** que existen archivos, comandos o carpetas mencionados como ejemplos en este documento. No reestructurar el repositorio ni actualizar dependencias masivamente como parte de un cambio puntual.
- Si falta información esencial, explicar la suposición utilizada y minimizar la superficie del cambio.

## 3. Fuentes de investigación

Estas fuentes son **referencias para estudiar**, no dependencias obligatorias ni garantías de que el protocolo actual funcione. Consultar fechas, issues, pull requests, commits, tags y licencias antes de confiar en una implementación.

| Fuente | Uso principal |
| --- | --- |
| https://github.com/mautrix/meta | Bridge de Messenger; sincronización, sesiones, cambios de protocolo y E2EE. Revisar especialmente issues, PR y releases. |
| https://github.com/gklka/Hermes | Cliente/bridge Messenger moderno con LightSpeed, DGW y E2EE. |
| https://github.com/MinhHuyDev/fbchat-v2 | Implementación Python; eventos, modelos de mensajes y aspectos de cifrado. |
| https://github.com/motovax/motofb | Implementación Go; HTTP/GraphQL, MQTT y recepción de eventos. |
| https://github.com/HerokeyVN/FB-Messenger-E2EE | Protocolo Signal y Noise en Messenger, `device-store`, prekeys y sockets E2EE. |
| https://github.com/jdcodes1/facebook-marketplace-mcp | Extracción de `doc_id` para operaciones GraphQL privadas de Facebook Marketplace. |
| https://github.com/VangBanLaNhat/fca-unofficial | Implementación JS; semántica FCA, sesiones y comportamiento MQTT. |
| https://github.com/TorMaireDusi/fca-unofficial | Experimentos de transporte y E2EE; contrastar con evidencia independiente. |
| https://github.com/dongp06/fca-unofficial | Referencia TypeScript histórica; **archivado**, no fuente primaria de cambios recientes. |
| https://github.com/fbchat-dev/fbchat | Diseño histórico de modelos/API; no asumir compatibilidad actual. |
| https://github.com/Schmavery/facebook-chat-api | Arquitectura FCA histórica; no asumir compatibilidad actual. |
| https://github.com/dequis/purple-facebook | Implementación histórica independiente del protocolo. |
| https://github.com/radian-software/unzuckify | Investigación de autenticación y GraphQL web. |
| https://cajum.github.io/fbgraphql/ | Investigación de esquemas privados de GraphQL/FlatBuffers; validar vigencia. |
| https://engineering.fb.com/2020/03/02/data-infrastructure/messenger/ | Contexto arquitectónico oficial de Project LightSpeed, **no** contrato de protocolo. |
| https://github.com/topics/facebook-messenger?o=desc&s=updated | Descubrir proyectos, forks y posibles nuevas fuentes; filtrar ruido. |

**Orden de confianza para cambios actuales:** (1) pruebas reproducibles de nuestra librería; (2) evidencia de cambios recientes en proyectos mantenidos e independientes; (3) código y conversaciones de issues/PR; (4) documentación técnica histórica. **Nunca** convertir una observación de un fork en verdad universal sin validar el flujo específico.

**Si el agente tiene acceso a Internet:** buscar antes de implementar un arreglo del protocolo; registrar URL exacta, fecha de consulta, fecha de commit/release y resultado observado. **Si no tiene acceso:** trabajar con el repositorio y pruebas locales; marcar cualquier hipótesis sobre el protocolo como *no verificada*. No inventar investigaciones o resultados.

## 4. Flujo de trabajo para cada cambio

### A. Definir el cambio

- Describir síntoma/feature, alcance, comportamiento esperado, versiones afectadas y efecto en la API pública.
- Determinar la capa: `auth/session`, `http/graphql`, `mqtt/websocket`, `lightspeed/sync`, `e2ee`, `parser/models`, `attachments`, `events` u otra identificada en el repositorio.
- Para bugs: intentar obtener una reproducción mínima con entradas **sintéticas o redactadas**. Para features: redactar criterios de aceptación y ejemplos de uso.

### B. Investigar y comparar

- Buscar el error, nombre de evento, código de operación o flujo en el repositorio y su historial.
- Consultar al menos **dos implementaciones independientes** cuando un cambio dependa de comportamiento privado de Messenger y sea viable hacerlo.
- Comparar la solución propuesta con nuestra arquitectura: formato de payload, semántica de eventos, orden, duplicados, reintentos, pérdida de conexión, compatibilidad E2EE y límites de cuenta.
- Registrar incertidumbre. Si las fuentes discrepan, no elegir a ciegas: aislar el comportamiento detrás de una interfaz o feature flag y añadir pruebas.
- Revisar la **licencia** de cualquier implementación externa. Tomar ideas de diseño y comportamiento; no copiar bloques de código, fixtures ni documentación de terceros sin que la licencia y las obligaciones de atribución lo permitan.

### C. Implementar

- Mantener transporte, parsing, estado y API pública desacoplados siempre que la estructura existente lo permita.
- Evitar constantes dispersas de endpoints, versiones, topics, códigos de operación y esquemas: centralizar los valores que el proyecto ya trate como configuración del protocolo.
- Usar parseo defensivo y errores tipados cuando el lenguaje/arquitectura lo permita; tratar campos desconocidos de forma segura y registrar solo metadatos no sensibles.
- Mantener idempotencia para reintentos y prevenir eventos duplicados; respetar orden, paginación y reconexiones cuando aplique.
- Preservar compatibilidad de llamadas existentes o indicar explícitamente un cambio incompatible con migración.
- No introducir una dependencia o refactorización global sin necesidad clara.

### D. Probar

- Añadir pruebas unitarias para transformaciones, validación, parsing y errores; tests de regresión para bugs.
- Probar entradas incompletas, eventos desconocidos, payloads inesperados, límites de tamaño, desconexiones y reintentos.
- Emplear mocks/fixtures **sintéticos, anonimizados o sanitizados**; no subir conversaciones, tokens, cookies o material criptográfico real.
- Ejecutar los comandos **existentes** de test, lint, typecheck y build. No afirmar que se ejecutaron si no fue así.
- Las pruebas de integración con cuentas reales son **opt-in**, requieren autorización explícita, secretos fuera del repositorio y deben minimizar envíos y acciones que afecten a otras personas. Nunca dispararlas automáticamente en CI público.

### E. Entregar y documentar

En la descripción del PR/cambio incluir:

1. **Problema y causa:** qué fallaba o qué se añade.
2. **Evidencia:** enlaces y fechas de fuentes consultadas; diferencias entre implementación observada e hipótesis.
3. **Archivos y API:** módulos tocados, compatibilidad y migración si aplica.
4. **Pruebas:** comandos ejecutados y resultado; limitaciones de validación.
5. **Riesgos y seguimiento:** casos desconocidos, posible rotura por cambios externos, señales para monitorizar.

Actualizar README, ejemplos, changelog y tipos públicos **si el cambio los afecta**.

## 5. Seguridad, privacidad y límites

- Las cookies de sesión, tokens, credenciales, claves, bases de datos locales y material E2EE son **secretos**. Nunca guardarlos en Git, logs, capturas, fixtures, reportes, mensajes de error o telemetría. Redactarlos antes de compartir evidencias.
- No introducir mecanismos para evadir CAPTCHA, MFA, bloqueos, límites de plataforma, verificaciones de identidad o controles de seguridad. Ante estos escenarios, ofrecer un error claro o una ruta de recuperación legítima.
- No implementar extracción masiva de datos, mensajes no autorizados ni automatizaciones de spam. Limitar pruebas a cuentas y conversaciones bajo autorización.
- No desactivar validación TLS ni aceptar certificados inválidos para solucionar conectividad.
- Minimizar persistencia de datos; usar cifrado/almacenamiento seguro de secretos cuando el entorno lo permita. No reinventar algoritmos criptográficos.
- Respetar términos de servicio y leyes aplicables; una implementación técnicamente posible no implica que esté autorizada. Explicitar riesgos de bloqueo de cuentas y de incompatibilidad.

## 6. Comportamientos que requieren cuidado especial

| Área | Verificar siempre |
| --- | --- |
| Login/sesiones | Expiración, renovación, recuperación legítima, cierre seguro, errores y protección de secretos. |
| MQTT/WebSocket | Handshake, topics, heartbeats, ack, reconexión, backoff, duplicados y orden de eventos. |
| GraphQL/HTTP | Identificadores de operación, parámetros, cabeceras, paginación, errores y cambios de esquema. |
| LightSpeed/sync | Estado incremental, cursor, replay, eventos desconocidos, consistencia e inicialización. |
| E2EE | Compatibilidad criptográfica, manejo de claves, estados de sesión, integridad, errores y no exposición de texto claro. |
| Adjuntos | Tamaño, tipo MIME, descargas parciales, validaciones, acceso autorizado y URLs temporales. |
| API pública | Tipos, compatibilidad, semántica de eventos, errores estables, ejemplos y versionado. |

## 7. Seguimiento continuo (si el proyecto dispone de CI)

- Revisar semanalmente releases, commits, issues y PR de referencias principales; aumentar vigilancia si aparecen errores nuevos en producción o fallos de integración.
- Clasificar novedades como `AUTH`, `MQTT`, `GRAPHQL`, `LIGHTSPEED`, `E2EE`, `ATTACHMENTS`, `EVENTS` o `SECURITY`.
- Registrar hallazgos con **fuente, fecha, síntoma, impacto, evidencia, urgencia y estado**. Distinguir “reportado por terceros” de “reproducido localmente”.
- Automatizar **detección y generación de reportes**, no copiar/aplicar parches de terceros ni realizar login con cuentas reales automáticamente.
- Ante un cambio incompatible: crear reproducción mínima, añadir test fallido, aplicar arreglo acotado y verificar regresión antes del merge.

## 8. Definición de terminado

Un cambio está terminado solo cuando:

- [ ] La necesidad está descrita y el alcance es limitado.
- [ ] Se ha contrastado el comportamiento privado con evidencia apropiada o se indica que falta verificarlo.
- [ ] Se han revisado seguridad, secretos, privacidad y licencias.
- [ ] Se han añadido/actualizado pruebas relevantes y ejecutado las comprobaciones disponibles.
- [ ] Se ha considerado compatibilidad y documentación de la API.
- [ ] El PR explica fuentes, resultados, limitaciones y riesgos restantes.

**Regla final:** investigar antes de adaptar, probar antes de afirmar y preservar la seguridad antes de intentar recuperar compatibilidad a cualquier costo.

Mantener los repositorios de investigacion bajo la carpeta de fbreego/docs/research.
los tests que se realizen no deben ejecutar la libreria sino la parte deseada si se encuentra en la fase de prueba.
antes de realizar cualquier cambio se debe actualizar el archivo de changelog.md con la descripcion del cambio y el numero de version y ultima fecha de consulta a estas librerias.
cuando se tenga que hacer un push se debe asegurar de que todo esta correcto y que no rompe ninguna funcionalidad.
realizar fallbacks seguros ante versiones anteriores en lugares donde sea compatible.
la carpeta examples debe estar actualizada con los ejemplos mas recientes.
busca en las librerias el mismo metodo o api para ver como se comunican y trata de entender como funciona la libreria para luego implementar ese mismo metodo en la nuestra.