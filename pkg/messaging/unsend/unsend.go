package unsend

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/bradez-rd/fbreego/pkg/core/httputil"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

const UnsendURL = "https://www.facebook.com/messaging/unsend_message/"

// UnsendMessage cancela el envío de un mensaje para todos los participantes de la conversación.
func UnsendMessage(ctx context.Context, client *http.Client, fb *types.DataFB, messageID string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	form := url.Values{
		"message_id": {messageID},
		"fb_dtsg":    {fb.FbDtsg},
		"jazoest":    {fb.Jazoest},
		"__user":     {fb.FacebookID},
		"av":         {fb.FacebookID},
		"__a":        {"1"},
		"__req":      {utils.NextRequestBase36()},
		"__rev":      {fb.ClientRevision},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", UnsendURL, strings.NewReader(form.Encode()))
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
		return nil, fmt.Errorf("error al cancelar envío de mensaje: %w", err)
	}
	defer resp.Body.Close()

	if err := httputil.CheckHTTPResponse(resp); err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return utils.ParseFacebookJSON(bodyBytes, true)
}
