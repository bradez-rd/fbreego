package traffic

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/logger"
)

type responseCaptureWriter struct {
	http.ResponseWriter
	statusCode int
	bodyBuf    bytes.Buffer
}

func (w *responseCaptureWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseCaptureWriter) Write(b []byte) (int, error) {
	if w.bodyBuf.Len() < 50*1024 {
		w.bodyBuf.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// InboundTrafficMiddleware captura las peticiones entrantes hacia la API REST del microservicio.
func InboundTrafficMiddleware(store *TrafficStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Excluir rutas de observabilidad o las peticiones del propio inspector de tráfico
			path := r.URL.Path
			if path == "/healthz" || path == "/readyz" || path == "/ws" ||
				strings.HasPrefix(path, "/api/v1/traffic") ||
				strings.HasPrefix(path, "/docs") || path == "/openapi.json" || path == "/" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Clonar body de la petición
			var reqBodyBytes []byte
			if r.Body != nil {
				reqBodyBytes, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewReader(reqBodyBytes))
			}

			rcw := &responseCaptureWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(rcw, r)

			duration := time.Since(start).Milliseconds()

			rawReqBody := string(reqBodyBytes)
			sanitizedReqBody := logger.RedactSensitive(rawReqBody)
			sanitizedReqHeaders := SanitizeHeaders(r.Header)

			rawRespBody := rcw.bodyBuf.String()
			if len(rawRespBody) > 50*1024 {
				rawRespBody = rawRespBody[:50*1024] + "... [truncado]"
			}
			sanitizedRespBody := logger.RedactSensitive(rawRespBody)

			record := &TrafficRecord{
				Timestamp:            start.UTC().Format(time.RFC3339),
				Direction:            DirectionInbound,
				Target:               "API",
				Method:               r.Method,
				URL:                  r.URL.RequestURI(),
				Path:                 r.URL.Path,
				Host:                 r.Host,
				StatusCode:           rcw.statusCode,
				StatusText:           http.StatusText(rcw.statusCode),
				DurationMs:           duration,
				RequestHeaders:       sanitizedReqHeaders,
				RequestBody:          sanitizedReqBody,
				RequestBodyFormatted: formatBodyPreview(sanitizedReqBody, r.Header.Get("Content-Type")),
				ResponseHeaders:      SanitizeHeaders(rcw.Header()),
				ResponseBody:         sanitizedRespBody,
				ResponseBodyFormat:   formatResponseBody(sanitizedRespBody, rcw.Header().Get("Content-Type")),
				CurlCommand:          GenerateCurl(r.Method, "http://localhost:8000"+r.URL.RequestURI(), sanitizedReqHeaders, sanitizedReqBody),
			}

			store.Record(record)
		})
	}
}
