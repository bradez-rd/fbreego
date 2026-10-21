package traffic

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/logger"
)

const bodyPreviewLimit = 64 * 1024

// TrafficTransport wraps outbound requests while retaining only bounded, redacted previews.
type TrafficTransport struct {
	base  http.RoundTripper
	store *TrafficStore
}

func NewTrafficTransport(base http.RoundTripper, store *TrafficStore) *TrafficTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &TrafficTransport{base: base, store: store}
}

func (t *TrafficTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.store == nil {
		return t.base.RoundTrip(req)
	}

	start := time.Now()
	var reqBodyBytes []byte
	// GetBody provides a separate reader for the common bytes/string reader cases and
	// avoids consuming a streaming request. Unknown streams are deliberately omitted.
	if req.GetBody != nil {
		if body, err := req.GetBody(); err == nil {
			reqBodyBytes, _ = io.ReadAll(io.LimitReader(body, bodyPreviewLimit+1))
			_ = body.Close()
		}
	}

	resp, reqErr := t.base.RoundTrip(req)
	duration := time.Since(start).Milliseconds()
	sanitizedReqHeaders := SanitizeHeaders(req.Header)
	sanitizedURL := SanitizeURL(req.URL.String())

	captureBodies := captureBodiesEnabled()
	sanitizedReqBody := "[Cuerpo omitido por privacidad - F-09]"
	if captureBodies {
		rawReqBody := previewString(reqBodyBytes)
		sanitizedReqBody = logger.RedactSensitive(rawReqBody)
	}

	target := "Facebook"
	if !strings.Contains(req.URL.Host, "facebook.com") {
		target = req.URL.Host
	}
	record := &TrafficRecord{
		Timestamp:            start.UTC().Format(time.RFC3339),
		Direction:            DirectionOutbound,
		Target:               target,
		Method:               req.Method,
		URL:                  sanitizedURL,
		Path:                 req.URL.Path,
		Host:                 req.URL.Host,
		DurationMs:           duration,
		RequestHeaders:       sanitizedReqHeaders,
		RequestBody:          sanitizedReqBody,
		RequestBodyFormatted: formatBodyPreview(sanitizedReqBody, req.Header.Get("Content-Type")),
		CurlCommand:          GenerateCurl(req.Method, sanitizedURL, sanitizedReqHeaders, sanitizedReqBody),
	}
	if reqErr != nil {
		record.StatusCode = http.StatusBadGateway
		record.StatusText = "Network Error"
		record.Error = reqErr.Error()
		t.store.Record(record)
		return nil, reqErr
	}

	sanitizedRespBody := "[Cuerpo omitido por privacidad - F-09]"
	if captureBodies && resp.Body != nil {
		var respBodyBytes []byte
		respBodyBytes, _ = io.ReadAll(io.LimitReader(resp.Body, bodyPreviewLimit+1))
		resp.Body = &compositeReadCloser{
			Reader: io.MultiReader(bytes.NewReader(respBodyBytes), resp.Body),
			Closer: resp.Body,
		}
		sanitizedRespBody = logger.RedactSensitive(previewString(respBodyBytes))
	}
	record.StatusCode = resp.StatusCode
	record.StatusText = resp.Status
	record.ResponseHeaders = SanitizeHeaders(resp.Header)
	record.ResponseBody = sanitizedRespBody
	record.ResponseBodyFormat = formatResponseBody(sanitizedRespBody, resp.Header.Get("Content-Type"))
	t.store.Record(record)
	return resp, nil
}

func captureBodiesEnabled() bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv("FBCHAT_TRAFFIC_CAPTURE_BODIES")))
	return val == "1" || val == "true" || val == "yes"
}

type compositeReadCloser struct {
	io.Reader
	io.Closer
}

func previewString(raw []byte) string {
	if len(raw) <= bodyPreviewLimit {
		return string(raw)
	}
	return string(raw[:bodyPreviewLimit]) + "... [truncated]"
}

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

func formatResponseBody(body, contentType string) string {
	cleaned := CleanFacebookJSON(body)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(cleaned), "", "  "); err == nil {
		return pretty.String()
	}
	return cleaned
}
