package traffic

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/logger"
)

// Direction representa si el tráfico es saliente hacia Facebook o entrante hacia el microservicio.
type Direction string

const (
	DirectionOutbound Direction = "OUTBOUND" // Librería -> Facebook
	DirectionInbound  Direction = "INBOUND"  // Cliente -> Microservicio API
)

// TrafficRecord contiene todos los detalles de una petición y su respuesta correspondiente.
type TrafficRecord struct {
	ID                   string            `json:"id"`
	Timestamp            string            `json:"timestamp"`
	Direction            Direction         `json:"direction"`
	Target               string            `json:"target"` // "Facebook" o "API"
	Method               string            `json:"method"`
	URL                  string            `json:"url"`
	Path                 string            `json:"path"`
	Host                 string            `json:"host"`
	StatusCode           int               `json:"statusCode"`
	StatusText           string            `json:"statusText"`
	DurationMs           int64             `json:"durationMs"`
	RequestHeaders       map[string]string `json:"requestHeaders"`
	RequestBody          string            `json:"requestBody"`
	RequestBodyFormatted string            `json:"requestBodyFormatted"`
	ResponseHeaders      map[string]string `json:"responseHeaders"`
	ResponseBody         string            `json:"responseBody"`
	ResponseBodyFormat   string            `json:"responseBodyFormatted"`
	Error                string            `json:"error,omitempty"`
	CurlCommand          string            `json:"curlCommand"`
}

// TrafficStats contiene métricas acumuladas del tráfico HTTP.
type TrafficStats struct {
	TotalRequests   int     `json:"totalRequests"`
	OutboundFB      int     `json:"outboundFb"`
	InboundAPI      int     `json:"inboundApi"`
	ErrorCount      int     `json:"errorCount"`
	AvgDurationMs   float64 `json:"avgDurationMs"`
	LastActivityUTC string  `json:"lastActivityUtc"`
}

// SanitizeHeaders filtra y oculta tokens, contraseñas y cookies confidenciales.
func SanitizeHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		joined := strings.Join(v, ", ")
		if strings.EqualFold(k, "cookie") || strings.EqualFold(k, "authorization") {
			result[k] = logger.RedactSensitive(joined)
		} else {
			result[k] = joined
		}
	}
	return result
}

// GenerateCurl crea un comando cURL listo para reproducir la petición en terminal.
func GenerateCurl(method, reqURL string, headers map[string]string, body string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("curl -X %s '%s'", method, reqURL))
	for k, v := range headers {
		if strings.EqualFold(k, "Accept-Encoding") {
			continue // omitir para evitar compresión en terminal
		}
		sb.WriteString(fmt.Sprintf(" \\\n  -H '%s: %s'", k, strings.ReplaceAll(v, "'", `\'`)))
	}
	if body != "" && method != "GET" && method != "HEAD" {
		cleanedBody := strings.ReplaceAll(body, "'", `\'`)
		if len(cleanedBody) > 2000 {
			cleanedBody = cleanedBody[:2000] + "... [truncado]"
		}
		sb.WriteString(fmt.Sprintf(" \\\n  --data '%s'", cleanedBody))
	}
	return sb.String()
}

// CleanFacebookJSON retira prefijos de seguridad como `for (;;);` para fácil inspección.
func CleanFacebookJSON(body string) string {
	trimmed := strings.TrimSpace(body)
	if strings.HasPrefix(trimmed, "for (;;);") {
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "for (;;);"))
	}
	return body
}

// UTCNow devuelve la hora UTC formateada en RFC3339.
func UTCNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}
