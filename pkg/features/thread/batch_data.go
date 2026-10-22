package thread

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/bradez-rd/fbreego/pkg/core/contracts"
	"github.com/bradez-rd/fbreego/pkg/core/httputil"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

// BatchQueryURL devuelve la URL canónica de GraphQLBatch configurada en el registro de contratos.
func BatchQueryURL() string {
	return contracts.BatchQueryURL()
}

// FetchAllThreads obtiene mediante una consulta GraphQLBatch la lista estructurada de conversaciones de la cuenta.
func FetchAllThreads(ctx context.Context, client *http.Client, fb *types.DataFB) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	docID := contracts.DocID("ThreadListQuery")
	queriesJSON := fmt.Sprintf(`{"o0":{"doc_id":"%s","query_params":{"limit":20,"before":null,"tags":["INBOX"]}}}`, docID)

	form := url.Values{
		"fb_dtsg":  {fb.FbDtsg},
		"jazoest":  {fb.Jazoest},
		"__user":   {fb.FacebookID},
		"__a":      {"1"},
		"__req":    {utils.NextRequestBase36()},
		"__rev":    {fb.ClientRevision},
		"queries":  {queriesJSON},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", contracts.BatchQueryURL(), strings.NewReader(form.Encode()))
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
		return nil, fmt.Errorf("error al consultar conversaciones batch: %w", err)
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
