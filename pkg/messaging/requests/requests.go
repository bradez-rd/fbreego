package requests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

const BatchURL = "https://www.facebook.com/api/graphqlbatch/"

// GetPendingRequests obtiene los hilos y mensajes en espera en la bandeja de solicitudes de Messenger.
func GetPendingRequests(ctx context.Context, client *http.Client, fb *types.DataFB) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	form := url.Values{
		"fb_dtsg":  {fb.FbDtsg},
		"jazoest":  {fb.Jazoest},
		"__user":   {fb.FacebookID},
		"__a":      {"1"},
		"__req":    {utils.NextRequestBase36()},
		"__rev":    {fb.ClientRevision},
		"queries":  {`{"o0":{"doc_id":"2289680371118670","query_params":{"folder_tag":"PENDING"}},"o1":{"doc_id":"2289680371118670","query_params":{"folder_tag":"OTHER"}}}`},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", BatchURL, strings.NewReader(form.Encode()))
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
		return nil, fmt.Errorf("error al consultar solicitudes de mensajes: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return utils.ParseFacebookJSON(bodyBytes, true)
}
