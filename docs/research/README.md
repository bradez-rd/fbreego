# Repositorios de Investigación — `docs/research/`

Este directorio contiene las fuentes de referencia, implementaciones independientes y herramientas de ingeniería inversa utilizadas para estudiar el protocolo no oficial de Facebook Messenger y guiar el desarrollo de **`fbreego`** en Go.

> **Regla de AGENTS.md (Líneas 138-144):**
> - Mantener los repositorios de investigación bajo la carpeta `fbreego/docs/research`.
> - Antes de implementar o modificar un método, buscar en estas librerías el método o API equivalente para comprender el flujo de comunicación, parámetros, endpoints y payloads, e implementarlo de forma nativa e idiomática en Go.
> - **Respetar licencias:** Las librerías de referencia son exclusivamente para estudio de protocolo y diseño comparativo. No se debe copiar código protegido por licencias restrictivas (como AGPL-3.0).
> - Los tests deben validar partes aisladas y controladas, sin ejecutar llamadas de red con credenciales reales no autorizadas.
> - Mantener actualizado `changelog.md` con la descripción del cambio y la fecha de última consulta a estas librerías.

---

## 1. Catálogo de Repositorios

| Directorio | Repositorio Upstream | Lenguaje | Licencia | Áreas Clave de Estudio |
| :--- | :--- | :--- | :--- | :--- |
| **`mautrix-meta`** | [`https://github.com/mautrix/meta`](https://github.com/mautrix/meta) | Go | AGPL-3.0 | Transporte DGW/LightSpeed, cursores de sincronización (`syncManager`), cliente HTTP con uTLS (`messagix`), reconexión MQTT y soporte E2EE. |
| **`motofb`** | [`https://github.com/motovax/motofb`](https://github.com/motovax/motofb) | Go | MIT | Implementación pura en Go de HTTP/GraphQL, sesión web/app, conexión MQTT y recepción de eventos en tiempo real. |
| **`fbchat_v2`** | [`https://github.com/MinhHuyDev/fbchat-v2`](https://github.com/MinhHuyDev/fbchat-v2) | Python | LGPL-3.0 | Contratos originales de fbchat v2.3.3: listeners, eventos, serialización, subida de adjuntos (Mercury), notas y features sociales. |
| **`fbchat`** | [`https://github.com/fbchat-dev/fbchat`](https://github.com/fbchat-dev/fbchat) | Python | BSD-3 | Diseño histórico de modelos, estructuras de hilos y arquitectura general de la API de Messenger. |
| **`fca-unofficial`** | [`https://github.com/VangBanLaNhat/fca-unofficial`](https://github.com/VangBanLaNhat/fca-unofficial) | JavaScript | MIT | Semántica FCA (Facebook Chat API), manejo de sesiones, rotación y refresco de `fb_dtsg`, inventario de acciones y listeners MQTT. |
| **`fb-messenger-e2ee`** | [`https://github.com/HerokeyVN/FB-Messenger-E2EE`](https://github.com/HerokeyVN/FB-Messenger-E2EE) | TypeScript | MIT / AGPL | Protocolo Signal y Noise en Messenger, `device-store`, prekeys, claves de sesión, descifrado de media cifrada y sockets Noise. |
| **`facebook-marketplace-mcp`** | [`https://github.com/jdcodes1/facebook-marketplace-mcp`](https://github.com/jdcodes1/facebook-marketplace-mcp) | TypeScript | MIT | Extracción de `doc_id` para operaciones GraphQL privadas de Facebook Marketplace, paginación, filtros de búsqueda y rate limiting. |

---

## 2. Flujo de Trabajo para Nuevas Funcionalidades o Correcciones

Cuando se requiera implementar un nuevo endpoint o corregir un problema de protocolo en `fbreego`:

1. **Localizar la operación equivalente:**
   - Buscar en `docs/research/mautrix-meta` (Go) o `docs/research/motofb` (Go) cómo envían la petición (ej. GraphQL `doc_id`, endpoint REST o paquete MQTT).
   - Revisar `docs/research/fbchat_v2` o `docs/research/fca-unofficial` para comparar parámetros (`av`, `fb_dtsg`, headers, cookies requeridas).
2. **Entender la respuesta y estructuras de datos:**
   - Identificar cómo se parsea la respuesta (JSON estándar, GraphQL batch payload o tareas SQLite LightSpeed).
3. **Escribir la solución nativa en `fbreego`:**
   - Ubicar la lógica en el paquete correspondiente (`pkg/core/lightspeed/`, `pkg/features/thread/`, `pkg/messaging/`, etc.).
   - Utilizar tipos Go fuertemente tipados y parseo defensivo ante campos faltantes.
4. **Verificar con pruebas:**
   - Crear pruebas unitarias con fixtures sintéticos en `*_test.go`.
   - Ejecutar la suite con `go test ./...`.

---

## 3. Mantenimiento y Actualización de Submódulos

Los repositorios externos están registrados como submódulos en `.gitmodules`.

Para inicializar o sincronizar los submódulos en un clon nuevo:
```bash
git submodule update --init --recursive
```

Para actualizar uno de los repositorios a su versión más reciente:
```bash
cd docs/research/<nombre-repositorio>
git pull origin main
cd ../../
git add docs/research/<nombre-repositorio>
```
