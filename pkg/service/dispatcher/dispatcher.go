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

	"github.com/bradez-rd/fbreego/pkg/core/logger"
	"github.com/bradez-rd/fbreego/pkg/service/webhook"
)

const webhookRecoveryInterval = 5 * time.Second

// WebhookTask representa una entrega pendiente en la cola asíncrona de webhooks.
type WebhookTask struct {
	JobID     string
	EventName string
	Payload   map[string]any
	Attempts  int
}

// Dispatcher gestiona el outbox cifrado, reintentos y entrega de eventos hacia servidores externos.
type Dispatcher struct {
	store          *webhook.WebhookConfigStore
	outbox         *webhook.WebhookOutboxStore
	client         *http.Client
	queue          chan WebhookTask
	queued         sync.Map
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
		outbox: webhook.NewWebhookOutboxStore(store.StateDir()),
		client: newSafeHTTPClient(),
		queue:  make(chan WebhookTask, 256),
	}
}

// Ready reporta si el outbox cifrado pudo inicializarse y recuperarse.
func (d *Dispatcher) Ready() error {
	if d == nil || d.outbox == nil {
		return fmt.Errorf("outbox de webhook no configurado")
	}
	return d.outbox.Ready()
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
	go d.recoveryLoop(workerCtx)
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

// Enqueue persiste un evento antes de programar su entrega asíncrona.
func (d *Dispatcher) Enqueue(eventName string, payload map[string]any) bool {
	if canonical, valid := webhook.CanonicalEventName(eventName); valid {
		eventName = canonical
	}
	job, err := d.outbox.Add(eventName, payload)
	if err != nil {
		d.totalFailed.Add(1)
		logger.Error("No se pudo persistir evento webhook %s: %v", eventName, err)
		return false
	}
	d.schedule(job)
	return true
}

func (d *Dispatcher) workerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-d.queue:
			err := d.deliverWithRetry(ctx, task.EventName, task.Payload)
			if err == nil {
				if deleteErr := d.outbox.Delete(task.JobID); deleteErr != nil {
					logger.Error("No se pudo confirmar entrega webhook %s: %v", task.JobID, deleteErr)
				}
			} else {
				nextAttempt := time.Now().Add(webhookRetryDelay(task.Attempts))
				if persistErr := d.outbox.MarkFailed(task.JobID, err, nextAttempt); persistErr != nil {
					logger.Error("No se pudo reprogramar entrega webhook %s: %v", task.JobID, persistErr)
				} else {
					logger.Error("Entrega webhook %s falló; próximo intento en %s: %v", task.JobID, nextAttempt.UTC().Format(time.RFC3339), err)
				}
			}
			d.queued.Delete(task.JobID)
		}
	}
}

func (d *Dispatcher) recoveryLoop(ctx context.Context) {
	d.scheduleDue()
	ticker := time.NewTicker(webhookRecoveryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.scheduleDue()
		}
	}
}

func (d *Dispatcher) scheduleDue() {
	for _, job := range d.outbox.Due(time.Now()) {
		d.schedule(job)
	}
}

func (d *Dispatcher) schedule(job webhook.WebhookJob) {
	if _, loaded := d.queued.LoadOrStore(job.ID, struct{}{}); loaded {
		return
	}
	select {
	case d.queue <- WebhookTask{
		JobID:     job.ID,
		EventName: job.EventName,
		Payload:   job.Payload,
		Attempts:  job.Attempts,
	}:
	default:
		d.queued.Delete(job.ID)
	}
}

func webhookRetryDelay(attempts int) time.Duration {
	delay := 5 * time.Second
	for attempt := 0; attempt < attempts && delay < 5*time.Minute; attempt++ {
		delay *= 2
	}
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

func (d *Dispatcher) deliverWithRetry(ctx context.Context, eventName string, payload map[string]any) error {
	cfg := d.store.Get()
	if !cfg.Enabled || cfg.URL == "" {
		return nil
	}
	if canonical, valid := webhook.CanonicalEventName(eventName); valid {
		eventName = canonical
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
		req.Header.Set("User-Agent", "fbreego/webhook-dispatcher")

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
		"message":   "fbreego test ping",
	}
	raw, _ := json.Marshal(probePayload)

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "fbreego/webhook-test")

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
	outboxReady := d.outbox != nil && d.outbox.Ready() == nil
	return map[string]any{
		"totalDelivered":    d.totalDelivered.Load(),
		"totalFailed":       d.totalFailed.Load(),
		"queueDepth":        len(d.queue),
		"persistentPending": d.outbox.Count(),
		"outboxReady":       outboxReady,
	}
}
