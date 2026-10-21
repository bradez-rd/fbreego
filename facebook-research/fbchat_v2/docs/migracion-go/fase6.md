# Fase 6 — Mensajería y Operaciones de Conversación

### A. Información General
- **Identificador:** FASE-06
- **Nombre de Fase:** Mensajería Messenger (Envío, Adjuntos, Reacciones, Edición, Unsend, Notas y Temas)
- **Objetivo:** Implementar todas las operaciones de mensajes convencionales en Messenger: envío de texto, carga multipart de archivos multimedia, citas y respuestas directas, reacciones con emoji, cancelación de envío (unsend), edición de mensajes ya enviados, administración de temas gráficos de chat y notas de 24 horas.
- **Prioridad:** Alta (P1 - Crítica)
- **Dependencias:** FASE-04, FASE-05.
- **Módulos originales involucrados:**
  - `fbchat_v2/_messaging/_send.py`
  - `fbchat_v2/_messaging/_attachments.py`
  - `fbchat_v2/_messaging/_reactions.py`
  - `fbchat_v2/_messaging/_unsend.py`
  - `fbchat_v2/_messaging/_editMessage.py`
  - `fbchat_v2/_messaging/_changeTheme.py`
  - `fbchat_v2/_messaging/_createNotes.py`
  - `fbchat_v2/_messaging/_message_requests.py`
- **Archivos Go previstos:**
  - `pkg/messaging/send/sender.go`
  - `pkg/messaging/attachments/uploader.go`
  - `pkg/messaging/reactions/reactions.go`
  - `pkg/messaging/unsend/unsend.go`
  - `pkg/messaging/edit/edit.go`
  - `pkg/messaging/theme/theme.go`
  - `pkg/messaging/notes/notes.go`
  - `pkg/messaging/requests/requests.go`
- **Estado inicial:** `PENDIENTE`

---

### B. Análisis del Código Original
- **Archivos y símbolos relevantes:**
  - `_send.py`: Endpoint `https://www.facebook.com/messaging/send/`. Requiere generar `otid` y `client_thread_id` numéricos, mapear `action_type: "ma-type:user-generated-message"`, serializar `has_attachment`, `body`, y si hay respuesta citada, el bloque `replied_to_message_id`.
  - `_attachments.py`: Endpoint `https://upload.facebook.com/ajax/mercury/upload.php`. Sube binario con `voice_clip: "true"` o `"false"` según el tipo de medio y extrae `metadata[0]["<type>_id"]` o `image_id`.
  - `_reactions.py`: Mutación GraphQL RelayModern (`doc_id` fijo) enviando variables `reaction`, `message_id`, `action: "ADD_REACTION"` o `"REMOVE_REACTION"`.
  - `_unsend.py`: POST a `https://www.facebook.com/messaging/unsend_message/` con `message_id`.
  - `_editMessage.py`: Publica tarea LightSpeed estructurada (`task: "edit_message"`) hacia el broker MQTT.
  - `_createNotes.py`: GraphQL queries y mutations para publicar notas efímeras de texto de hasta 60 caracteres y audios breves.
- **Riesgos específicos:**
  - Cambios en los `doc_id` de GraphQL utilizados por Facebook para notas o reacciones.
  - Límites de tamaño de adjuntos y tipos MIME no admitidos.

---

### C. Plan de Implementación
- **Arquitectura propuesta:** Módulos desacoplados que consumen el cliente HTTP base y la sesión `DataFB`.
- **Interfaces y estructuras Go:**
  ```go
  type MessageSender interface {
      SendText(ctx context.Context, fb *types.DataFB, req SendTextRequest) (*SendResponse, error)
      SendWithAttachments(ctx context.Context, fb *types.DataFB, req SendMediaRequest) (*SendResponse, error)
  }

  type AttachmentUploader interface {
      Upload(ctx context.Context, fb *types.DataFB, filename string, r io.Reader) (*AttachmentResult, error)
  }
  ```

---

### D. Tareas Ejecutables
- [ ] Implementar `SendText` con generación de IDs `otid` y soporte de `replyToMessageId`.
- [ ] Implementar el cargador multipart `UploadAttachment` para imágenes, audio, video y documentos.
- [ ] Implementar `ReactMessage` (añadir y retirar reacciones emoji vía RelayModern GraphQL).
- [ ] Implementar `UnsendMessage` hacia el endpoint de Mercury unsend.
- [ ] Implementar `EditMessage` coordinado con la publicación de tareas LightSpeed.
- [ ] Implementar `ChangeTheme`, `ListThemes` y `FindTheme`.
- [ ] Implementar `CreateNote`, `CheckNote`, `DeleteNote` y `RecreateNote`.
- [ ] Implementar `GetPendingRequests` para consultar la bandeja de solicitudes.

---

### E. Matriz de Equivalencia
| Componente original | Referencia de origen | Equivalente Go | Prueba | Estado |
|---|---|---|---|---|
| `send` | `_messaging/_send.py:api.send` | `pkg/messaging/send.SendText()` | Test con mock server | PENDIENTE |
| `upload` | `_messaging/_attachments.py:upload` | `pkg/messaging/attachments.Upload()` | Test de multipart upload | PENDIENTE |
| `react` | `_messaging/_reactions.py:func` | `pkg/messaging/reactions.React()` | Test de mutación GraphQL | PENDIENTE |
| `unsend` | `_messaging/_unsend.py:func` | `pkg/messaging/unsend.Unsend()` | Test de petición unsend | PENDIENTE |
| `editMessage` | `_messaging/_editMessage.py:editMessage` | `pkg/messaging/edit.EditMessage()` | Test de tarea LS | PENDIENTE |
| `notes` | `_messaging/_createNotes.py:createNote` | `pkg/messaging/notes.CreateNote()` | Test de mutación notas | PENDIENTE |
| `theme` | `_messaging/_changeTheme.py:changeTheme` | `pkg/messaging/theme.ChangeTheme()` | Test de temas | PENDIENTE |

---

### F. Pruebas y Aceptación
- Pruebas unitarias de ensamblado de formularios contra capturas reales de peticiones.
- Prueba de carga de adjuntos multipart con streams en memoria (`bytes.Buffer`).
- Validación de manejo de errores de Facebook (ej. `error_code: 1545012` por acción bloqueada).

---

### G. Riesgos y Dependencias
- Coordinación entre edición de mensajes y la sesión MQTT de la Fase 7.

---

### H. Definición de Terminado
La fase se considerará terminada cuando todas las operaciones de mensajes, notas y adjuntos estén implementadas y verificadas con pruebas de integración.
