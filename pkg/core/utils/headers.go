package utils

import (
	"net/http"
	"os"
	"strings"
)

// DefaultUserAgent define el User-Agent estable y coherente predeterminado (Windows 10 Chrome).
const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

const secChUA = `"Chromium";v="140", "Not=A?Brand";v="24", "Google Chrome";v="140"`

// ClientUserAgent devuelve una identidad de User-Agent estable para la sesión,
// permitiendo sobrescritura controlada mediante FBCHAT_USER_AGENT.
func ClientUserAgent() string {
	if ua := strings.TrimSpace(os.Getenv("FBCHAT_USER_AGENT")); ua != "" {
		return ua
	}
	return DefaultUserAgent
}

// RandomUserAgent devuelve el User-Agent configurado de forma estable para evitar
// contradicciones técnicas entre peticiones sucesivas de la misma sesión.
func RandomUserAgent() string {
	return ClientUserAgent()
}

// PlatformForUserAgent deduce la plataforma correspondiente para la cabecera sec-ch-ua-platform.
func PlatformForUserAgent(ua string) string {
	lower := strings.ToLower(ua)
	switch {
	case strings.Contains(lower, "macintosh") || strings.Contains(lower, "mac os"):
		return `"macOS"`
	case strings.Contains(lower, "linux") && !strings.Contains(lower, "android"):
		return `"Linux"`
	case strings.Contains(lower, "android"):
		return `"Android"`
	default:
		return `"Windows"`
	}
}

// DefaultHeaders genera las cabeceras HTTP coherentes con el navegador emulado para Facebook.
func DefaultHeaders(host string) http.Header {
	if host == "" {
		host = "www.facebook.com"
	}
	ua := ClientUserAgent()
	platform := PlatformForUserAgent(ua)

	h := make(http.Header)
	h.Set("Connection", "keep-alive")
	h.Set("User-Agent", ua)
	h.Set("Accept", "*/*")
	h.Set("Origin", "https://"+host)
	h.Set("Sec-Fetch-Site", "same-origin")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("Sec-Fetch-Dest", "empty")
	// Nota de seguridad F-05: Sec-Fetch-User solo aplica a navegaciones (?1), se omite deliberadamente en CORS
	h.Set("x-asbd-id", "129477")
	h.Set("Referer", "https://"+host+"/")
	h.Set("sec-ch-ua", secChUA)
	h.Set("sec-ch-ua-mobile", "?0")
	h.Set("sec-ch-ua-platform", platform)
	h.Set("Accept-Language", "en-US,en;q=0.9")
	return h
}

// ApplyHeaders asigna DefaultHeaders a una petición http.Request y configura req.Host correctamente.
func ApplyHeaders(req *http.Request, host string) {
	if req == nil {
		return
	}
	if host == "" {
		host = "www.facebook.com"
	}
	req.Host = host
	headers := DefaultHeaders(host)
	for k, v := range headers {
		req.Header[k] = v
	}
}
