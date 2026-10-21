# Fase 8 — Multimedia y Funcionalidades Adicionales

### A. Información General
- **Identificador:** FASE-08
- **Nombre de Fase:** Funcionalidades de Red Social, Administración de Hilos y Gestión de Medios Locales
- **Objetivo:** Migrar todas las capacidades no relacionadas directamente con el envío básico de mensajes: acciones del perfil de Facebook (posts, bio, bloqueo, marketplace, búsqueda, modo profesional, amigos) y administración de hilos de chat (administradores, nombres, emojis, apodos y batch de metadatos), así como el gestor de almacenamiento temporal de medios.
- **Prioridad:** Media (P2)
- **Dependencias:** FASE-04, FASE-05.
- **Módulos originales involucrados:**
  - `fbchat_v2/_features/_facebook/` (13 módulos)
  - `fbchat_v2/_features/_thread/` (5 módulos)
  - `fbchat_v2/service_media.py`
- **Archivos Go previstos:**
  - `pkg/features/social/posts.go`
  - `pkg/features/social/profile.go`
  - `pkg/features/social/blocking.go`
  - `pkg/features/social/marketplace.go`
  - `pkg/features/social/search.go`
  - `pkg/features/social/notifications.go`
  - `pkg/features/thread/admin.go`
  - `pkg/features/thread/customization.go`
  - `pkg/features/thread/batch_data.go`
  - `pkg/service/media/storage.go`
- **Estado inicial:** `PENDIENTE`

---

### B. Análisis del Código Original
- **Archivos y símbolos relevantes:**
  - `_features/_thread/_all_thread_data.py`: Ejecuta GraphQL Batch Request a `https://www.facebook.com/api/graphqlbatch/` solicitando consultas múltiples de bandeja de entrada, carpetas (inbox, spam, archivados), mensajes recientes y participantes.
  - `_features/_thread/_changeEmoji.py`, `_changeNameThread.py`, `_changeNickname.py`, `_addAdmin.py`: Operaciones administrativas de grupos a través de endpoints AJAX (`/messaging/save_thread_emoji/`, `/ajax/mercury/change_thread_name.php`, etc.).
  - `_features/_facebook/_marketplace.py`: Publica anuncios en Marketplace con título, precio, categoría, coordenadas geográficas de ubicación (`latitude`, `longitude`) y consulta detalles por `item_id`.
  - `_features/_facebook/_createPost.py`, `_deletePost.py`, `_archivePost.py`, `_reactionPost.py`: Ciclo de vida de publicaciones en el feed.
  - `service_media.py:MediaStorage`: Almacena archivos cargados en disco, calcula su hash SHA-256 para evitar duplicados, sanea nombres de archivo y provee métodos de consulta y borrado.
- **Riesgos específicos:**
  - Detección de patrones de scraping en Facebook Marketplace o búsqueda masiva de usuarios.

---

### C. Plan de Implementación
- **Arquitectura propuesta:** Agrupación lógica en dos paquetes Go (`pkg/features/social` y `pkg/features/thread`) con clientes que implementan interfaces limpias.
- **Interfaces y estructuras Go:**
  ```go
  type ThreadManager interface {
      ChangeName(ctx context.Context, fb *types.DataFB, threadID, name string) error
      ChangeEmoji(ctx context.Context, fb *types.DataFB, threadID, emoji string) error
      ChangeNickname(ctx context.Context, fb *types.DataFB, threadID, userID, nickname string) error
      ManageAdmin(ctx context.Context, fb *types.DataFB, threadID, adminID string, add bool) error
      FetchAllThreads(ctx context.Context, fb *types.DataFB) (*ThreadsDataResult, error)
  }

  type SocialManager interface {
      CreatePost(ctx context.Context, fb *types.DataFB, text string) (*PostResult, error)
      GetUserInfo(ctx context.Context, fb *types.DataFB, userID string) (*UserInfo, error)
      SearchUsers(ctx context.Context, fb *types.DataFB, query string) ([]UserSearchResult, error)
      BlockUser(ctx context.Context, fb *types.DataFB, userID string, block bool) error
      GetMarketplaceItem(ctx context.Context, fb *types.DataFB, itemID string) (*MarketplaceItem, error)
  }
  ```

---

### D. Tareas Ejecutables
- [ ] Implementar la consulta GraphQL Batch de `FetchAllThreads` con extracción de participantes y configuración de notificaciones.
- [ ] Implementar la administración de hilos (`ChangeName`, `ChangeEmoji`, `ChangeNickname`, `ManageAdmin`).
- [ ] Implementar publicaciones de muro: `CreatePost`, `DeletePost`, `ArchivePost`, `ReactPost`.
- [ ] Implementar gestión de perfil y biografía: `ChangeBio`, `ToggleProfessionalMode`, `RegisterProfile`.
- [ ] Implementar seguridad y privacidad: `BlockUser`, `Unfriend`.
- [ ] Implementar consultas: `GetUserInfo`, `SearchUsers`, `GetNotifications`.
- [ ] Implementar Facebook Marketplace: `CreateMarketplaceItem` y `GetMarketplaceItem`.
- [ ] Implementar `MediaStorage` en Go con deduplicación por SHA-256 y control de concurrencia.

---

### E. Matriz de Equivalencia
| Componente original | Referencia de origen | Equivalente Go | Prueba | Estado |
|---|---|---|---|---|
| `_all_thread_data` | `_features/_thread/_all_thread_data.py:func` | `pkg/features/thread.FetchAllThreads()` | Test de GraphQL batch | PENDIENTE |
| `_changeNameThread` | `_features/_thread/_changeNameThread.py:func` | `pkg/features/thread.ChangeThreadName()` | Test de renombrado | PENDIENTE |
| `_get_user_info` | `_features/_facebook/_get_user_info.py:func` | `pkg/features/social.GetUserInfo()` | Test de perfil | PENDIENTE |
| `_marketplace` | `_features/_facebook/_marketplace.py:createItem` | `pkg/features/social.CreateMarketplaceItem()` | Test de item marketplace | PENDIENTE |
| `MediaStorage` | `service_media.py:MediaStorage` | `pkg/service/media.Storage` | Test de hash y deduplicación | PENDIENTE |

---

### F. Pruebas y Aceptación
- Validación de parsing de estructuras complejas de GraphQL Batch de hilos.
- Pruebas unitarias de almacenamiento de archivos multimedia con verificación de hash SHA-256.

---

### G. Riesgos y Dependencias
- Control de cuotas de subida en `MediaStorage` para evitar llenado accidental de disco.

---

### H. Definición de Terminado
La fase se considerará terminada cuando todas las funciones de red social, administración de grupos y almacenamiento multimedia pasen sus pruebas unitarias.
