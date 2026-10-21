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
	writePreview(&w.bodyBuf, b)
	return w.ResponseWriter.Write(b)
}

type limitedCapture struct{ buf bytes.Buffer }

func (w *limitedCapture) Write(p []byte) (int, error) {
	writePreview(&w.buf, p)
	return len(p), nil
}

func writePreview(dst *bytes.Buffer, p []byte) {
	remaining := bodyPreviewLimit - dst.Len()
	if remaining <= 0 {
		return
	}
	if remaining < len(p) {
		p = p[:remaining]
	}
	_, _ = dst.Write(p)
}

type teeReadCloser struct {
	io.Reader
	io.Closer
}

func InboundTrafficMiddleware(store *TrafficStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store == nil {
				next.ServeHTTP(w, r)
				return
			}
			path := r.URL.Path
			if path == "/healthz" || path == "/readyz" || path == "/ws" ||
				strings.HasPrefix(path, "/api/v1/traffic") ||
				strings.HasPrefix(path, "/docs") || path == "/openapi.json" || path == "/" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			capture := &limitedCapture{}
			if r.Body != nil {
				r.Body = &teeReadCloser{Reader: io.TeeReader(r.Body, capture), Closer: r.Body}
			}
			rcw := &responseCaptureWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rcw, r)

			sanitizedReqBody := logger.RedactSensitive(capture.buf.String())
			sanitizedReqHeaders := SanitizeHeaders(r.Header)
			sanitizedRespBody := logger.RedactSensitive(rcw.bodyBuf.String())
			sanitizedURL := SanitizeURL("http://localhost:8000" + r.URL.RequestURI())
			record := &TrafficRecord{
				Timestamp:            start.UTC().Format(time.RFC3339),
				Direction:            DirectionInbound,
				Target:               "API",
				Method:               r.Method,
				URL:                  sanitizedURL,
				Path:                 r.URL.Path,
				Host:                 r.Host,
				StatusCode:           rcw.statusCode,
				StatusText:           http.StatusText(rcw.statusCode),
				DurationMs:           time.Since(start).Milliseconds(),
				RequestHeaders:       sanitizedReqHeaders,
				RequestBody:          sanitizedReqBody,
				RequestBodyFormatted: formatBodyPreview(sanitizedReqBody, r.Header.Get("Content-Type")),
				ResponseHeaders:      SanitizeHeaders(rcw.Header()),
				ResponseBody:         sanitizedRespBody,
				ResponseBodyFormat:   formatResponseBody(sanitizedRespBody, rcw.Header().Get("Content-Type")),
				CurlCommand:          GenerateCurl(r.Method, sanitizedURL, sanitizedReqHeaders, sanitizedReqBody),
			}
			store.Record(record)
		})
	}
}
