package httputil

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

// NewDefaultTransport crea un http.Transport optimizado para reutilización de sockets y emulación de navegador.
func NewDefaultTransport(proxyURL string) *http.Transport {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	} else if envProxy := os.Getenv("HTTPS_PROXY"); envProxy != "" {
		if u, err := url.Parse(envProxy); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	} else if envHTTPProxy := os.Getenv("HTTP_PROXY"); envHTTPProxy != "" {
		if u, err := url.Parse(envHTTPProxy); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}

	return transport
}

// NewClient crea un http.Client listo para producción con tiempos límite y transporte configurado.
func NewClient(timeout time.Duration, proxyURL string) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{
		Transport: NewDefaultTransport(proxyURL),
		Timeout:   timeout,
	}
}

// Sentinel errors para clasificación centralizada de respuestas del proveedor.
var (
	ErrSessionExpired    = errors.New("sesión expirada o no autorizada (HTTP 401)")
	ErrAccountRestricted = errors.New("cuenta bloqueada, con restricciones o checkpoint requerido (HTTP 403)")
	ErrRateLimited       = errors.New("límite de tasa excedido por el proveedor (HTTP 429)")
	ErrUpstreamFailure   = errors.New("fallo interno en el servidor del proveedor (HTTP 5xx)")
)

// CheckHTTPResponse clasifica el código de estado HTTP y extrae Retry-After si corresponde.
func CheckHTTPResponse(resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("respuesta http nula")
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return ErrSessionExpired
	case http.StatusForbidden:
		return ErrAccountRestricted
	case http.StatusTooManyRequests:
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			return fmt.Errorf("%w (Retry-After: %s)", ErrRateLimited, retryAfter)
		}
		return ErrRateLimited
	default:
		if resp.StatusCode >= 500 {
			return fmt.Errorf("%w: código %d", ErrUpstreamFailure, resp.StatusCode)
		}
		return fmt.Errorf("facebook respondió con código http inesperado: %d", resp.StatusCode)
	}
}

