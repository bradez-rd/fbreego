package listener

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

var ErrTransportUnavailable = errors.New("el transporte MQTT de Messenger no está implementado")

// MQTTListener implementa la conexión activa y escucha de eventos de mensajería mediante MQTT sobre WebSockets de Facebook.
type MQTTListener struct {
	fb          *types.DataFB
	syncMgr     *SyncManager
	mu          sync.Mutex
	running     bool
	cancel      context.CancelFunc
	messageChan chan DeltaMessage
	client      mqtt.Client
	syncToken   string
}

// NewMQTTListener crea un nuevo gestor de escucha MQTT para una sesión de Facebook.
func NewMQTTListener(fb *types.DataFB) *MQTTListener {
	return &MQTTListener{
		fb:          fb,
		syncMgr:     NewSyncManager(0),
		messageChan: make(chan DeltaMessage, 1000),
	}
}

// Messages expone el canal de lectura para eventos de mensajes entrantes.
func (l *MQTTListener) Messages() <-chan DeltaMessage { return l.messageChan }

// Start inicia la conexión MQTT sobre WebSocket con edge-chat.facebook.com.
func (l *MQTTListener) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		return fmt.Errorf("el listener ya se encuentra en ejecución")
	}
	if l.fb == nil {
		return ErrTransportUnavailable
	}

	sessionID := utils.GenerateSessionID()
	brokerURL := fmt.Sprintf("wss://edge-chat.facebook.com/chat?region=eag&sid=%d", sessionID)

	userPayload := map[string]any{
		"u":          l.fb.FacebookID,
		"s":          sessionID,
		"chat_on":    "true",
		"fg":         false,
		"d":          utils.GenerateClientID(),
		"ct":         "websocket",
		"aid":        219994525426954,
		"mqtt_sid":   "",
		"cp":         3,
		"ecp":        10,
		"st":         "/t_ms",
		"pm":         []any{},
		"dc":         "",
		"no_auto_fg": true,
		"gas":        nil,
		"pack":       []any{},
	}
	userBytes, _ := json.Marshal(userPayload)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID("mqttwsclient")
	opts.SetUsername(string(userBytes))
	opts.SetProtocolVersion(3) // MQTT 3.1
	opts.SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12})
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(30 * time.Second)
	opts.SetKeepAlive(10 * time.Second)

	headers := http.Header{}
	headers.Set("Cookie", l.fb.CookieFacebook)
	headers.Set("Origin", "https://www.facebook.com")
	headers.Set("User-Agent", utils.ConsistentUserAgent)
	headers.Set("Referer", "https://www.facebook.com/")
	headers.Set("Host", "edge-chat.facebook.com")
	opts.SetHTTPHeaders(headers)

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		l.publishSyncQueue(c)
	})

	opts.SetDefaultPublishHandler(func(c mqtt.Client, m mqtt.Message) {
		l.handleIncomingMessage(m)
	})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if token.WaitTimeout(5*time.Second) && token.Error() != nil {
		// Logueamos o permitimos reintento en background sin bloquear
	}

	l.client = client
	l.running = true
	return nil
}

func (l *MQTTListener) publishSyncQueue(c mqtt.Client) {
	seqID := l.syncMgr.GetLastSeqID()
	queue := map[string]any{
		"sync_api_version":           10,
		"max_deltas_able_to_process": 1000,
		"delta_batch_size":           500,
		"encoding":                   "JSON",
		"entity_fbid":                l.fb.FacebookID,
		"orca_version":               "1.2.0",
	}

	var topic string
	if l.syncToken == "" {
		topic = "/messenger_sync_create_queue"
		queue["initial_titan_sequence_id"] = seqID
		queue["device_params"] = nil
	} else {
		topic = "/messenger_sync_get_diffs"
		queue["last_seq_id"] = seqID
		queue["sync_token"] = l.syncToken
	}

	queueBytes, _ := json.Marshal(queue)
	c.Publish(topic, 1, false, queueBytes)
}

func (l *MQTTListener) handleIncomingMessage(m mqtt.Message) {
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(m.Payload()))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return
	}

	if st := stringValue(payload["syncToken"]); st != "" {
		l.syncToken = st
	}
	if seqID := int64Value(firstValue(payload, "lastIssuedSeqId", "firstDeltaSeqId")); seqID > 0 {
		l.syncMgr.UpdateSeqID(seqID)
	}

	for _, event := range decodePayloadEvents(payload) {
		l.DispatchIncoming(event)
	}
}

const metaAIThreadID = "156025504001094"

// decodePayloadEvents converts the JSON shapes produced by Messenger MQTT and
// by the E2EE bridge into the provider's canonical realtime message events.
// Keeping this conversion pure makes protocol regressions testable without a
// live Facebook account.
func decodePayloadEvents(payload map[string]any) []DeltaMessage {
	var events []DeltaMessage
	seen := make(map[string]struct{})

	appendEvent := func(event DeltaMessage) {
		if event.MessageID == "" || (event.EventType == MessageEdited && strings.TrimSpace(event.Body) == "") {
			return
		}
		if event.EventType == "" {
			event.EventType = MessageReceived
		}
		key := strings.Join([]string{string(event.EventType), event.MessageID, event.ThreadID, event.Body}, "\x00")
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		events = append(events, event)
	}

	var visit func(map[string]any, bool)
	visit = func(node map[string]any, forcedEdit bool) {
		eventName := strings.ToLower(stringValue(firstValue(node, "event", "type", "kind")))
		isEdit := forcedEdit || eventName == "messageedit" || eventName == "message_edit" ||
			eventName == "message.edited" || eventName == "edit"

		content := node
		if nested, ok := node["data"].(map[string]any); ok && (isEdit || node["messageMetadata"] == nil) {
			content = nested
		}

		if isEdit {
			appendEvent(messageFromNode(content, MessageEdited))
		} else if _, ok := content["messageMetadata"].(map[string]any); ok {
			appendEvent(messageFromNode(content, MessageReceived))
		}

		for _, key := range []string{"deltaMessageEdit", "messageEdit", "message_edit"} {
			if nested, ok := node[key].(map[string]any); ok {
				visit(nested, true)
			}
		}
		if reply, ok := node["deltaMessageReply"].(map[string]any); ok {
			if message, ok := reply["message"].(map[string]any); ok {
				visit(message, false)
			}
		}
		if nested, ok := node["payload"].(map[string]any); ok {
			visit(nested, false)
		} else if raw := strings.TrimSpace(stringValue(node["payload"])); strings.HasPrefix(raw, "{") {
			var nested map[string]any
			decoder := json.NewDecoder(strings.NewReader(raw))
			decoder.UseNumber()
			if decoder.Decode(&nested) == nil {
				visit(nested, false)
			}
		}
		if deltas, ok := node["deltas"].([]any); ok {
			for _, rawDelta := range deltas {
				if delta, ok := rawDelta.(map[string]any); ok {
					visit(delta, false)
				}
			}
		}
	}

	visit(payload, false)
	return events
}

func messageFromNode(node map[string]any, eventType MessageEventType) DeltaMessage {
	metadata, _ := node["messageMetadata"].(map[string]any)
	threadKey, _ := metadata["threadKey"].(map[string]any)
	if threadKey == nil {
		threadKey, _ = node["threadKey"].(map[string]any)
	}

	messageID := stringValue(firstNonEmptyValue(node, "messageId", "messageID", "targetId", "id"))
	if messageID == "" {
		messageID = stringValue(firstNonEmptyValue(metadata, "messageId", "messageID"))
	}
	senderID := stringValue(firstNonEmptyValue(node, "senderId", "senderID", "actorId", "actorID"))
	if senderID == "" {
		senderID = stringValue(firstNonEmptyValue(metadata, "actorFbId", "actorId"))
	}
	threadID := stringValue(firstNonEmptyValue(node, "threadId", "threadID", "chatId", "chatJid"))
	otherUserID := stringValue(firstNonEmptyValue(threadKey, "otherUserFbId", "otherUserId"))
	threadFBID := stringValue(firstNonEmptyValue(threadKey, "threadFbId", "threadId"))
	if threadID == "" {
		if otherUserID != "" {
			threadID = otherUserID
		} else {
			threadID = threadFBID
		}
	}

	threadType := stringValue(firstNonEmptyValue(node, "threadType", "thread_type"))
	if threadType == "" {
		threadType = stringValue(firstNonEmptyValue(metadata, "threadType", "thread_type"))
	}
	isGroup := threadFBID != "" && otherUserID == ""
	if isDirectThread(threadType, threadID, senderID) {
		isGroup = false
	}

	body := stringValue(firstNonEmptyValue(node, "newText", "text", "body", "messageContent"))
	timestamp := int64Value(firstNonEmptyValue(node, "timestampMs", "timestamp", "editedAt"))
	if timestamp == 0 {
		timestamp = int64Value(firstNonEmptyValue(metadata, "timestampMs", "timestamp"))
	}

	return DeltaMessage{
		EventType:   eventType,
		MessageID:   messageID,
		ThreadID:    threadID,
		SenderID:    senderID,
		Body:        body,
		TimestampMs: timestamp,
		IsGroup:     isGroup,
		ThreadType:  threadType,
	}
}

func isDirectThread(threadType, threadID, senderID string) bool {
	normalized := strings.ToLower(strings.TrimSpace(threadType))
	switch normalized {
	case "1", "7", "10", "13", "15", "201", "user", "one_to_one", "ai_bot", "aibot":
		return true
	}
	return threadID == metaAIThreadID || (threadID != "" && threadID == senderID)
}

func firstValue(values map[string]any, keys ...string) any {
	if values == nil {
		return nil
	}
	for _, key := range keys {
		if value, exists := values[key]; exists {
			return value
		}
	}
	return nil
}

func firstNonEmptyValue(values map[string]any, keys ...string) any {
	if values == nil {
		return nil
	}
	for _, key := range keys {
		if value, exists := values[key]; exists && stringValue(value) != "" {
			return value
		}
	}
	return nil
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	default:
		return ""
	}
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		return 0
	}
}

// Stop finaliza la escucha y cierra el cliente MQTT.
func (l *MQTTListener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running {
		return
	}
	if l.client != nil && l.client.IsConnected() {
		l.client.Disconnect(250)
	}
	if l.cancel != nil {
		l.cancel()
	}
	l.running = false
}

// DispatchIncoming envía de forma segura un mensaje decodificado a la cola de recepción.
func (l *MQTTListener) DispatchIncoming(msg DeltaMessage) {
	select {
	case l.messageChan <- msg:
	default:
	}
}

// GetLastSeqID devuelve el número de secuencia más reciente.
func (l *MQTTListener) GetLastSeqID() int64 { return l.syncMgr.GetLastSeqID() }

// ClientID genera un identificador de cliente.
func (l *MQTTListener) ClientID() string { return utils.GenerateClientID() }
