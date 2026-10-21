package httputil

import (
	"crypto/tls"
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
