package dispatcher

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bradez-rd/fbreego/pkg/service/webhook"
)

// WebhookTask representa una entrega pendiente en la cola asíncrona de webhooks.
type WebhookTask struct {
	EventName string
	Payload   map[string]any
}

// Dispatcher gestiona la cola en memoria, reintentos y entrega de eventos hacia servidores externos.
type Dispatcher struct {
	store          *webhook.WebhookConfigStore
	client         *http.Client
	queue          chan WebhookTask
	mu             sync.Mutex
	running        bool
	cancel         context.CancelFunc
	totalDelivered atomic.Int64
	totalFailed    atomic.Int64
}

// NewDispatcher crea una nueva instancia del despachador de webhooks.
func NewDispatcher(store *webhook.WebhookConfigStore) *Dispatcher {
	return &Dispatcher{
		store:  store,
		client: newSafeHTTPClient(),
		queue:  make(chan WebhookTask, 256),
	}
}

// Start inicia el worker pool del despachador en background.
func (d *Dispatcher) Start(ctx context.Context) {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel
	d.running = true
	d.mu.Unlock()

	go d.workerLoop(workerCtx)
}

// Stop detiene ordenadamente el procesamiento de la cola de webhooks.
func (d *Dispatcher) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return
	}
	if d.cancel != nil {
		d.cancel()
	}
	d.running = false
}

// Enqueue añade un evento a la cola de despacho asíncrona.
func (d *Dispatcher) Enqueue(eventName string, payload map[string]any) bool {
	select {
	case d.queue <- WebhookTask{EventName: eventName, Payload: payload}:
		return true
	default:
		d.totalFailed.Add(1)
		return false
	}
}

func (d *Dispatcher) workerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-d.queue:
			_ = d.deliverWithRetry(ctx, task.EventName, task.Payload)
		}
	}
}

func (d *Dispatcher) deliverWithRetry(ctx context.Context, eventName string, payload map[string]any) error {
	cfg := d.store.Get()
	if !cfg.Enabled || cfg.URL == "" {
		return nil
	}

	// Verificar si el evento está en la lista de eventos suscritos
	if len(cfg.Events) > 0 {
		subscribed := false
		for _, e := range cfg.Events {
			if e == eventName || e == "*" {
				subscribed = true
				break
			}
		}
		if !subscribed {
			return nil
		}
	}

	if IsSSRFRisk(cfg.URL) {
		d.totalFailed.Add(1)
		return fmt.Errorf("la URL del webhook viola las políticas de protección anti-SSRF")
	}

	body := map[string]any{
		"event":     eventName,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"payload":   payload,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	// Reintentos exponenciales (máximo 3 intentos)
	var lastErr error
	backoff := 500 * time.Millisecond
	for attempt := 0; attempt < 3; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, "POST", cfg.URL, bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "fbchat-provider/webhook-dispatcher")

		if cfg.Secret != "" {
			mac := hmac.New(sha256.New, []byte(cfg.Secret))
			mac.Write(raw)
			sig := hex.EncodeToString(mac.Sum(nil))
			req.Header.Set("X-Hub-Signature-256", "sha256="+sig)
		}

		resp, err := d.client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				d.totalDelivered.Add(1)
				return nil
			}
			lastErr = fmt.Errorf("servidor webhook devolvió código HTTP %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
		backoff *= 2
	}

	d.totalFailed.Add(1)
	return lastErr
}

// TestProbe ejecuta un sondeo síncrono para verificar conectividad y respuesta del servidor webhook.
func (d *Dispatcher) TestProbe(ctx context.Context, targetURL, secret string) (map[string]any, error) {
	if targetURL == "" {
		cfg := d.store.Get()
		targetURL = cfg.URL
		if secret == "" {
			secret = cfg.Secret
		}
	}

	if targetURL == "" {
		return nil, fmt.Errorf("no hay una URL de webhook configurada")
	}

	if IsSSRFRisk(targetURL) {
		return nil, fmt.Errorf("la URL destino fue bloqueada por el filtro anti-SSRF")
	}

	probePayload := map[string]any{
		"event":     "webhook.test",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"message":   "fbchat-provider test ping",
	}
	raw, _ := json.Marshal(probePayload)

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "fbchat-provider/webhook-test")

	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(raw)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Hub-Signature-256", "sha256="+sig)
	}

	start := time.Now()
	resp, err := d.client.Do(req)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return nil, fmt.Errorf("fallo al contactar webhook: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	return map[string]any{
		"statusCode": resp.StatusCode,
		"durationMs": duration,
		"response":   string(respBytes),
		"success":    resp.StatusCode >= 200 && resp.StatusCode < 300,
	}, nil
}

// Stats devuelve métricas acumuladas del despachador.
func (d *Dispatcher) Stats() map[string]any {
	return map[string]any{
		"totalDelivered": d.totalDelivered.Load(),
		"totalFailed":    d.totalFailed.Load(),
		"queueDepth":     len(d.queue),
	}
}
