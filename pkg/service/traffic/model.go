package traffic

import (
	"fmt"
	"net/http"
	"net/url"
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
	ReqHeaders           map[string]string `json:"reqHeaders,omitempty"`
	RequestBody          string            `json:"requestBody"`
	ReqBody              string            `json:"reqBody,omitempty"`
	ReqBodySize          int               `json:"reqBodySize,omitempty"`
	RequestBodyFormatted string            `json:"requestBodyFormatted"`
	ResponseHeaders      map[string]string `json:"responseHeaders"`
	RespHeaders          map[string]string `json:"respHeaders,omitempty"`
	ResponseBody         string            `json:"responseBody"`
	RespBody             string            `json:"respBody,omitempty"`
	RespBodySize         int               `json:"respBodySize,omitempty"`
	ResponseBodyFormat   string            `json:"responseBodyFormatted"`
	Error                string            `json:"error,omitempty"`
	CurlCommand          string            `json:"curlCommand"`
	SessionID            string            `json:"sessionId,omitempty"`
	TenantID             string            `json:"tenantId,omitempty"`
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
		if isSensitiveHeader(k) {
			result[k] = "<redacted>"
		} else {
			result[k] = logger.RedactSensitive(joined)
		}
	}
	return result
}

func isSensitiveHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "proxy-authorization", "cookie", "set-cookie", "x-api-key", "x-auth-token":
		return true
	default:
		return false
	}
}

// SanitizeURL removes credentials and sensitive query values before persistence.
func SanitizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "<invalid-url>"
	}
	parsed.User = nil
	query := parsed.Query()
	for key := range query {
		switch strings.ToLower(key) {
		case "token", "access_token", "secret", "password", "authorization", "fb_dtsg", "xs", "c_user", "ticket":
			query.Set(key, "<redacted>")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
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
