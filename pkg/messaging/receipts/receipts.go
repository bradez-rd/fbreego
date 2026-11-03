package receipts

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

const (
	ChangeReadStatusURL = "https://www.facebook.com/ajax/mercury/change_read_status.php"
	DeliveryReceiptsURL = "https://www.facebook.com/ajax/mercury/delivery_receipts.php"
	TypingURL           = "https://www.facebook.com/ajax/messaging/typ.php"
)

// MarkRead envía a Facebook el acuse de lectura de un hilo.
func MarkRead(ctx context.Context, client *http.Client, fb *types.DataFB, threadID string, watermark int64) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if watermark <= 0 {
		watermark = time.Now().UnixMilli()
	}

	form := url.Values{
		"fb_dtsg":                {fb.FbDtsg},
		"jazoest":                {fb.Jazoest},
		"__user":                 {fb.FacebookID},
		"__a":                    {"1"},
		"__req":                  {utils.NextRequestBase36()},
		"__rev":                  {fb.ClientRevision},
		"ids[" + threadID + "]":  {"true"},
		"watermarkTimestamp":     {strconv.FormatInt(watermark, 10)},
		"shouldSendReadReceipt":  {"true"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", ChangeReadStatusURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	headers := utils.DefaultHeaders("www.facebook.com")
	for k, v := range headers {
		req.Header[k] = v
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fb.CookieFacebook)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error al marcar lectura en Facebook: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	res, err := utils.ParseFacebookJSON(bodyBytes, true)
	if err != nil {
		// Fallback suave si no devuelve JSON estricto pero el status HTTP es 200
		if resp.StatusCode == http.StatusOK {
			return map[string]any{"status": "read", "threadId": threadID, "watermark": watermark}, nil
		}
		return nil, err
	}
	res["threadId"] = threadID
	res["watermark"] = watermark
	return res, nil
}

// MarkDelivered envía a Facebook el acuse de entrega de mensajes.
func MarkDelivered(ctx context.Context, client *http.Client, fb *types.DataFB, threadID string, watermark int64) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if watermark <= 0 {
		watermark = time.Now().UnixMilli()
	}

	form := url.Values{
		"fb_dtsg":            {fb.FbDtsg},
		"jazoest":            {fb.Jazoest},
		"__user":             {fb.FacebookID},
		"__a":                {"1"},
		"__req":              {utils.NextRequestBase36()},
		"__rev":              {fb.ClientRevision},
		"thread_fbid":        {threadID},
		"watermarkTimestamp": {strconv.FormatInt(watermark, 10)},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", DeliveryReceiptsURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	headers := utils.DefaultHeaders("www.facebook.com")
	for k, v := range headers {
		req.Header[k] = v
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fb.CookieFacebook)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error al marcar entrega en Facebook: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	res, err := utils.ParseFacebookJSON(bodyBytes, true)
	if err != nil {
		if resp.StatusCode == http.StatusOK {
			return map[string]any{"status": "delivered", "threadId": threadID, "watermark": watermark}, nil
		}
		return nil, err
	}
	res["threadId"] = threadID
	res["watermark"] = watermark
	return res, nil
}

// SendTyping envía el indicador de escritura ("typing.started" o "typing.stopped") a Facebook.
func SendTyping(ctx context.Context, client *http.Client, fb *types.DataFB, threadID string, isTyping bool) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	typVal := "0"
	if isTyping {
		typVal = "1"
	}

	form := url.Values{
		"fb_dtsg":  {fb.FbDtsg},
		"jazoest":  {fb.Jazoest},
		"__user":   {fb.FacebookID},
		"__a":      {"1"},
		"__req":    {utils.NextRequestBase36()},
		"__rev":    {fb.ClientRevision},
		"typ":      {typVal},
		"to":       {threadID},
		"source":   {"mercury-chat"},
		"thread":   {threadID},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", TypingURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	headers := utils.DefaultHeaders("www.facebook.com")
	for k, v := range headers {
		req.Header[k] = v
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fb.CookieFacebook)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error al enviar indicador de typing: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	res, err := utils.ParseFacebookJSON(bodyBytes, true)
	if err != nil {
		if resp.StatusCode == http.StatusOK {
			return map[string]any{"status": "confirmed", "threadId": threadID, "isTyping": isTyping}, nil
		}
		return nil, err
	}
	res["threadId"] = threadID
	res["isTyping"] = isTyping
	return res, nil
}
