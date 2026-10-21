package thread

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

const AddAdminURL = "https://www.facebook.com/messaging/save_admins/"

// ManageAdmin añade o retira permisos de administrador a un participante de un grupo de Messenger.
func ManageAdmin(ctx context.Context, client *http.Client, fb *types.DataFB, threadID, adminID string, add bool) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	statusChoice := "false"
	if add {
		statusChoice = "true"
	}

	form := url.Values{
		"thread_fbid":             {threadID},
		"admin_ids[0]":            {adminID},
		"add":                     {statusChoice},
		"fb_dtsg":                 {fb.FbDtsg},
		"jazoest":                 {fb.Jazoest},
		"__user":                  {fb.FacebookID},
		"__a":                     {"1"},
		"__req":                   {utils.NextRequestBase36()},
		"__rev":                   {fb.ClientRevision},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", AddAdminURL, strings.NewReader(form.Encode()))
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
		return nil, fmt.Errorf("error al modificar administrador de grupo: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return utils.ParseFacebookJSON(bodyBytes, true)
}
