# Fase 7 — Eventos en Tiempo Real, Protocolo MQTT y Cifrado E2EE

### A. Información General
- **Identificador:** FASE-07
- **Nombre de Fase:** Escucha en Tiempo Real (MQTT edge-chat), Gestor de Secuencias, Hub WebSocket y Cifrado E2EE
- **Objetivo:** Implementar la conexión de tiempo real a los brokers de Messenger mediante MQTT sobre WebSocket seguro, el seguimiento atómico de números de secuencia (`seq_id`), la integración del protocolo de cifrado de extremo a extremo (E2EE) y el Hub Pub/Sub WebSocket para distribución de eventos en memoria.
- **Prioridad:** Alta (P1 - Crítica)
- **Dependencias:** FASE-04, FASE-05, FASE-06.
- **Módulos originales involucrados:**
  - `fbchat_v2/_messaging/_listening.py`
  - `fbchat_v2/_messaging/_listening_e2ee.py`
  - `fbchat_v2/_messaging/_bridge_actions.py`
  - `fbchat_v2/_messaging/_send_e2ee.py`
  - `fbchat_v2/service_events.py`
- **Archivos Go previstos:**
  - `pkg/messaging/listener/mqtt_client.go`
  - `pkg/messaging/listener/sync.go`
  - `pkg/messaging/e2ee/protocol.go`
  - `pkg/messaging/e2ee/actions.go`
  - `pkg/messaging/e2ee/media.go`
  - `pkg/messaging/e2ee/redaction.go`
  - `pkg/service/events/hub.go`
  - `pkg/service/events/listener_manager.go`
- **Estado inicial:** `PENDIENTE`

---

### B. Análisis del Código Original
- **Archivos y símbolos relevantes:**
  - `listeningEvent`: Conecta con Paho MQTT sobre `wss://edge-chat.facebook.com/chat`.
    - Tópicos suscritos: `/t_ms`, `/thread_typing`, `/orca_presence`, `/orca_message_notifications`.
    - Manejo de deltas: desempaqueta deltas de mensajes entrantes, extrae `body`, `sender_id`, `thread_key`, `message_id`, `attachments`.
    - Control de sincronización: `_coerce_seq_id` y `_set_last_seq_id` garantizan que no se procesen eventos duplicados tras reconexiones.
  - `_BridgeProcess` y `BridgeActions`:
    - Gestiona comunicación JSON-RPC bidireccional sobre stdin/stdout con el proceso de cifrado E2EE.
    - Acciones: `editE2EEMessage`, `unsendE2EEMessage`, `sendTypingIndicator`, `markRead`, `sendE2EETyping`, `sendE2EEAudio`, `sendE2EEImage`, `downloadE2EEMedia`.
    - Filtro de redacción en logs: oculta tokens, cookies, identificadores sensibles y payloads.
  - `RealtimeEventHub`: Mantiene clientes WebSocket conectados, emite eventos tipados (`session_status`, `message_received`, `raw_event`) y reenvía a `WebhookDispatcher`.
  - `SessionListenerManager`: Controla la ejecución, reinicio y terminación del worker de cada sesión.
- **Riesgos específicos:**
  - Desconexiones silenciosas en MQTT por parte de los firewalls de Meta si no se envían paquetes `PINGREQ` a tiempo (keep-alive de 60 s).
  - Bloqueos de goroutines en clientes WebSocket lentos o desconectados abruptamente.

---

### C. Plan de Implementación
- **Arquitectura propuesta:**
  - Cliente MQTT basado en `github.com/eclipse/paho.golang` con reconexión exponencial y canal de eventos tipados.
  - Modo E2EE dual: integración directa de código Go de cifrado o proceso IPC JSON-RPC configurable.
  - `RealtimeHub` implementado con canales con buffer (`chan Event`, cap=256) por cliente y expulsión de clientes lentos (*slow consumer drop*).
- **Interfaces y estructuras Go:**
  ```go
  type RealtimeListener interface {
      Start(ctx context.Context) error
      Stop() error
      Events() <-chan Event
  }

  type E2EEBridge interface {
      Call(ctx context.Context, method string, params any, result any) error
      Close() error
  }
  ```

---

### D. Tareas Ejecutables
- [ ] Implementar cliente MQTT v3.1.1 sobre WebSocket seguro con `edge-chat.facebook.com`.
- [ ] Implementar decodificador de payloads delta (`/t_ms`) y seguimiento atómico de `seq_id`.
- [ ] Implementar `RealtimeEventHub` con registro concurrente de conexiones WebSocket.
- [ ] Implementar `SessionListenerManager` para gestionar el ciclo de vida del listener de cada sesión con `context.CancelFunc`.
- [ ] Implementar cliente `E2EEBridge` (modo nativo y modo IPC para `BridgeActions`).
- [ ] Implementar acciones E2EE: `EditE2EEMessage`, `UnsendE2EEMessage`, `SendE2EEAudio`, `SendE2EEImage`, `DownloadE2EEMedia`.
- [ ] Implementar filtro de redacción de secretos en logs (`RedactSensitiveData`).

---

### E. Matriz de Equivalencia
| Componente original | Referencia de origen | Equivalente Go | Prueba | Estado |
|---|---|---|---|---|
| `listeningEvent` | `_messaging/_listening.py:listeningEvent` | `pkg/messaging/listener.MQTTListener` | Test de suscripción MQTT | PENDIENTE |
| `_set_last_seq_id` | `_messaging/_listening.py:_set_last_seq_id` | `pkg/messaging/listener.SyncManager` | Test de orden de seq_id | PENDIENTE |
| `BridgeActions` | `_messaging/_bridge_actions.py:BridgeActions` | `pkg/messaging/e2ee.Actions` | Test de llamadas RPC | PENDIENTE |
| `RealtimeEventHub` | `service_events.py:RealtimeEventHub` | `pkg/service/events.RealtimeHub` | Test de difusión WebSocket | PENDIENTE |
| `SessionListenerManager` | `service_events.py:SessionListenerManager` | `pkg/service/events.ListenerManager` | Test de ciclo de vida | PENDIENTE |

---

### F. Pruebas y Aceptación
- Prueba de desconexión y reconexión automática en el cliente MQTT con reanudación en el último `seq_id`.
- Prueba de broadcast con 50 clientes WebSocket conectados simulando recepción de eventos simultáneos.
- Verificación con `-race` de que no existan condiciones de carrera en el hub de eventos.

---

### G. Riesgos y Dependencias
- Coordinación con el microservicio web de la Fase 9 para la conexión de tickets WebSocket.

---

### H. Definición de Terminado
La fase se considerará terminada cuando los listeners MQTT reciban eventos simulados, el puente E2EE procese acciones y el hub WebSocket distribuya los mensajes a clientes conectados.
