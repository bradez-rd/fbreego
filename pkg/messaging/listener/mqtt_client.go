package listener

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

// MQTTListener gestiona la conexión de tiempo real a los servidores edge-chat de Facebook.
type MQTTListener struct {
	fb          *types.DataFB
	syncMgr     *SyncManager
	mu          sync.Mutex
	running     bool
	cancel      context.CancelFunc
	messageChan chan DeltaMessage
}

// NewMQTTListener crea un nuevo listener de eventos en tiempo real.
func NewMQTTListener(fb *types.DataFB) *MQTTListener {
	return &MQTTListener{
		fb:          fb,
		syncMgr:     NewSyncManager(0),
		messageChan: make(chan DeltaMessage, 100),
	}
}

// Messages devuelve el canal receptor de mensajes entrantes.
func (l *MQTTListener) Messages() <-chan DeltaMessage {
	return l.messageChan
}

// Start inicia el bucle de escucha de eventos en tiempo real.
func (l *MQTTListener) Start(ctx context.Context) error {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return fmt.Errorf("el listener ya se encuentra en ejecución")
	}

	listenerCtx, cancel := context.WithCancel(ctx)
	l.cancel = cancel
	l.running = true
	l.mu.Unlock()

	go l.listenLoop(listenerCtx)
	return nil
}

// Stop finaliza la conexión y el procesamiento del listener.
func (l *MQTTListener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running {
		return
	}
	if l.cancel != nil {
		l.cancel()
	}
	l.running = false
}

func (l *MQTTListener) listenLoop(ctx context.Context) {
	// Reintento con backoff exponencial
	backoff := 1 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Simulación de escucha controlada por contexto mientras se enlaza el broker
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
	}
}

// DispatchIncoming emite un mensaje decodificado hacia los suscriptores.
func (l *MQTTListener) DispatchIncoming(msg DeltaMessage) {
	select {
	case l.messageChan <- msg:
	default:
		// Canal lleno, omitir para no bloquear el loop del broker
	}
}

// GetLastSeqID devuelve el número de secuencia actual.
func (l *MQTTListener) GetLastSeqID() int64 {
	return l.syncMgr.GetLastSeqID()
}

// ClientID genera un identificador de cliente MQTT consistente.
func (l *MQTTListener) ClientID() string {
	return utils.GenerateClientID()
}
