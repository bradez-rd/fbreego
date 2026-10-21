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

const (
	ChangeNameURL     = "https://www.facebook.com/messaging/set_thread_name/"
	ChangeEmojiURL    = "https://www.facebook.com/messaging/save_thread_emoji/"
	ChangeNicknameURL = "https://www.facebook.com/messaging/save_thread_nickname/"
)

// ChangeThreadName actualiza el título o nombre visible de un grupo en Messenger.
func ChangeThreadName(ctx context.Context, client *http.Client, fb *types.DataFB, threadID, newName string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	form := url.Values{
		"thread_name": {newName},
		"thread_id":   {threadID},
		"fb_dtsg":     {fb.FbDtsg},
		"jazoest":     {fb.Jazoest},
		"__user":      {fb.FacebookID},
		"__a":         {"1"},
		"__req":       {utils.NextRequestBase36()},
		"__rev":       {fb.ClientRevision},
	}

	return postForm(ctx, client, fb, ChangeNameURL, form)
}

// ChangeEmoji cambia el emoji representativo de una conversación en Messenger.
func ChangeEmoji(ctx context.Context, client *http.Client, fb *types.DataFB, threadID, emoji string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	form := url.Values{
		"emoji_choice": {emoji},
		"thread_or_other_fbid": {threadID},
		"fb_dtsg":      {fb.FbDtsg},
		"jazoest":      {fb.Jazoest},
		"__user":       {fb.FacebookID},
		"__a":          {"1"},
		"__req":        {utils.NextRequestBase36()},
		"__rev":        {fb.ClientRevision},
	}

	return postForm(ctx, client, fb, ChangeEmojiURL, form)
}

// ChangeNickname asigna o borra el apodo de un participante en una conversación de Messenger.
func ChangeNickname(ctx context.Context, client *http.Client, fb *types.DataFB, threadID, participantID, nickname string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	form := url.Values{
		"nickname":             {nickname},
		"participant_id":       {participantID},
		"thread_or_other_fbid": {threadID},
		"fb_dtsg":              {fb.FbDtsg},
		"jazoest":              {fb.Jazoest},
		"__user":               {fb.FacebookID},
		"__a":                  {"1"},
		"__req":                {utils.NextRequestBase36()},
		"__rev":                {fb.ClientRevision},
	}

	return postForm(ctx, client, fb, ChangeNicknameURL, form)
}

func postForm(ctx context.Context, client *http.Client, fb *types.DataFB, endpoint string, form url.Values) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
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
		return nil, fmt.Errorf("error al ejecutar petición a %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return utils.ParseFacebookJSON(bodyBytes, true)
}
