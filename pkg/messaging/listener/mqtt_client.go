package listener

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

var ErrTransportUnavailable = errors.New("el transporte MQTT de Messenger no está implementado")

type MQTTListener struct {
	fb          *types.DataFB
	syncMgr     *SyncManager
	mu          sync.Mutex
	running     bool
	cancel      context.CancelFunc
	messageChan chan DeltaMessage
}

func NewMQTTListener(fb *types.DataFB) *MQTTListener {
	return &MQTTListener{fb: fb, syncMgr: NewSyncManager(0), messageChan: make(chan DeltaMessage, 100)}
}

func (l *MQTTListener) Messages() <-chan DeltaMessage { return l.messageChan }

// Start fails explicitly until the real Messenger transport is available.
func (l *MQTTListener) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		return fmt.Errorf("el listener ya se encuentra en ejecución")
	}
	return ErrTransportUnavailable
}

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

func (l *MQTTListener) DispatchIncoming(msg DeltaMessage) {
	select {
	case l.messageChan <- msg:
	default:
	}
}

func (l *MQTTListener) GetLastSeqID() int64 { return l.syncMgr.GetLastSeqID() }
func (l *MQTTListener) ClientID() string    { return utils.GenerateClientID() }
