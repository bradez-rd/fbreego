package traffic

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/logger"
)

// TrafficTransport envuelve un http.RoundTripper para interceptar y registrar peticiones salientes a Facebook.
type TrafficTransport struct {
	base  http.RoundTripper
	store *TrafficStore
}

// NewTrafficTransport crea un nuevo transporte instrumentado.
func NewTrafficTransport(base http.RoundTripper, store *TrafficStore) *TrafficTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &TrafficTransport{
		base:  base,
		store: store,
	}
}

// RoundTrip intercepta la petición saliente, registra los datos y devuelve la respuesta intacta al llamador.
func (t *TrafficTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.store == nil {
		return t.base.RoundTrip(req)
	}

	start := time.Now()

	// 1. Clonar el body de la petición
	var reqBodyBytes []byte
	if req.Body != nil {
		var err error
		reqBodyBytes, err = io.ReadAll(req.Body)
		if err == nil {
			req.Body = io.NopCloser(bytes.NewReader(reqBodyBytes))
		}
	}

	// 2. Ejecutar la petición HTTP real
	resp, reqErr := t.base.RoundTrip(req)
	duration := time.Since(start).Milliseconds()

	// 3. Procesar datos de la petición
	rawReqBody := string(reqBodyBytes)
	sanitizedReqBody := logger.RedactSensitive(rawReqBody)
	formattedReqBody := formatBodyPreview(sanitizedReqBody, req.Header.Get("Content-Type"))
	sanitizedReqHeaders := SanitizeHeaders(req.Header)

	target := "Facebook"
	if !strings.Contains(req.URL.Host, "facebook.com") {
		target = req.URL.Host
	}

	record := &TrafficRecord{
		Timestamp:            start.UTC().Format(time.RFC3339),
		Direction:            DirectionOutbound,
		Target:               target,
		Method:               req.Method,
		URL:                  req.URL.String(),
		Path:                 req.URL.Path,
		Host:                 req.URL.Host,
		DurationMs:           duration,
		RequestHeaders:       sanitizedReqHeaders,
		RequestBody:          sanitizedReqBody,
		RequestBodyFormatted: formattedReqBody,
		CurlCommand:          GenerateCurl(req.Method, req.URL.String(), sanitizedReqHeaders, sanitizedReqBody),
	}

	// 4. Si hubo error a nivel de transporte/red
	if reqErr != nil {
		record.StatusCode = http.StatusBadGateway
		record.StatusText = "Network Error"
		record.Error = reqErr.Error()
		t.store.Record(record)
		return nil, reqErr
	}

	// 5. Leer y restaurar el body de respuesta
	var respBodyBytes []byte
	if resp.Body != nil {
		respBodyBytes, _ = io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewReader(respBodyBytes))
	}

	record.StatusCode = resp.StatusCode
	record.StatusText = resp.Status
	record.ResponseHeaders = SanitizeHeaders(resp.Header)

	rawRespBody := string(respBodyBytes)
	// Truncar respuestas muy grandes para no saturar memoria
	if len(rawRespBody) > 100*1024 {
		rawRespBody = rawRespBody[:100*1024] + "... [truncado por tamaño]"
	}

	sanitizedRespBody := logger.RedactSensitive(rawRespBody)
	record.ResponseBody = sanitizedRespBody
	record.ResponseBodyFormat = formatResponseBody(sanitizedRespBody, resp.Header.Get("Content-Type"))

	t.store.Record(record)
	return resp, nil
}

// formatBodyPreview intenta estructurar el payload (form-data o json) en formato legible.
func formatBodyPreview(body, contentType string) string {
	if body == "" {
		return ""
	}
	if strings.Contains(contentType, "application/json") {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, []byte(body), "", "  "); err == nil {
			return pretty.String()
		}
	}
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		values, err := url.ParseQuery(body)
		if err == nil {
			var sb strings.Builder
			for k, v := range values {
				sb.WriteString(k)
				sb.WriteString(": ")
				sb.WriteString(strings.Join(v, ", "))
				sb.WriteString("\n")
			}
			return strings.TrimSpace(sb.String())
		}
	}
	return body
}

// formatResponseBody limpia el prefijo de Facebook `for (;;);` e indenta JSON.
func formatResponseBody(body, contentType string) string {
	cleaned := CleanFacebookJSON(body)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(cleaned), "", "  "); err == nil {
		return pretty.String()
	}
	return cleaned
}
