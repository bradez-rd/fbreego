fbreego/
├── bin/
│ └── fbreego.exe # Binario autocontenido compilado
├── cmd/
│ └── fbreego/
│ └── main.go # Punto de entrada de la CLI
├── main.go # Entrypoint raíz compatible
├── web/
│ ├── embed.go # Incrustación estática go:embed
│ └── portal.html # SPA administrativo reactivo
├── pkg/
│ ├── core/ # Infraestructura base y bajo nivel
│ │ ├── auth/ # Login nativo FB4A y generador RFC 6238 TOTP
│ │ ├── console/ # Salida de consola UTF-8 segura
│ │ ├── httputil/ # Cliente HTTP optimizado, Keep-Alive y proxies
│ │ ├── lightspeed/ # Parser de scripts de inicialización Messenger
│ │ ├── logger/ # Logger con redacción automática de secretos
│ │ ├── permissions/ # Permisos restrictivos (Windows ACLs y POSIX 0600)
│ │ ├── session/ # Extracción de tokens (fb_dtsg, jazoest, actorID)
│ │ ├── storage/ # Contratos de sesión (File, Env, Memory)
│ │ ├── types/ # DTOs, modelos Pydantic y respuestas unificadas
│ │ └── utils/ # Generación de IDs, Base36, headers y parser JSON
│ ├── messaging/ # Lógica de mensajería de Facebook Messenger
│ │ ├── attachments/ # Subida multipart a Mercury con deduplicación
│ │ ├── e2ee/ # Puente y acciones de protocolo cifrado E2EE
│ │ ├── edit/ # Tareas LightSpeed para edición de mensajes
│ │ ├── listener/ # Cliente MQTT y sincronizador atómico de secuencias
│ │ ├── notes/ # CRUD de notas efímeras de 24 horas
│ │ ├── reactions/ # Reacciones emoji a mensajes
│ │ ├── requests/ # Bandeja de solicitudes de mensajes (graphqlbatch)
│ │ ├── send/ # Envío de texto y respuestas citadas
│ │ ├── theme/ # Consulta y asignación de temas gráficos
│ │ └── unsend/ # Cancelación de envío (Unsend) para todos
│ ├── features/ # Características sociales y de grupos
│ │ ├── social/ # Publicaciones, bio, modo profesional, bloqueo, búsqueda
│ │ └── thread/ # Administración de grupos, apodos, nombres y emoji
│ └── service/ # Microservicio REST y WebSocket
│ ├── api/ # 37+ rutas HTTP chi/v5, Swagger OpenAPI y portal
│ ├── dispatcher/ # Despachador asíncrono de webhooks con HMAC y anti-SSRF
│ ├── events/ # Hub WebSocket Pub/Sub y pumps de escritura
│ ├── media/ # Almacén local de medios con deduplicación SHA-256
│ ├── runner/ # Ciclo de vida del servidor y apagado gracioso
│ ├── sessions/ # Almacén de sesiones cifrado AES-256-GCM y registro
│ └── webhook/ # Gestión de configuraciones de webhook
└── facebook-research/fbchat_v2/docs/migracion-go/
├── README.md # Índice general de documentación
├── ARQUITECTURA_GO.md # Diagramas y decisiones de diseño
├── INVENTARIO_FUNCIONAL.md # Inventario de los 63 archivos analizados
├── MATRIZ_COMPATIBILIDAD.md # Trazabilidad de las 80 capacidades
└── fase1.md ... fase11.md # Documentación detallada por fase
