package social

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

const UnfriendURL = "https://www.facebook.com/ajax/profile/removefriendconfirm.php"

// BlockUser bloquea o desbloquea a un usuario en Facebook y Messenger.
func BlockUser(ctx context.Context, client *http.Client, fb *types.DataFB, targetUserID string, block bool) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	action := "UNBLOCK"
	if block {
		action = "BLOCK"
	}

	variables := map[string]any{
		"input": map[string]any{
			"blockee_id": targetUserID,
			"action": action,
			"actor_id": fb.FacebookID,
			"client_mutation_id": utils.NextRequestBase36(),
		},
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "PrivacyBlockUserMutation", "3829104820491023", true)
	form.Set("variables", string(varBytes))

	return executePost(ctx, client, fb, GraphQLURL, form)
}

// UnfriendUser remueve a un contacto de la lista de amigos en Facebook.
func UnfriendUser(ctx context.Context, client *http.Client, fb *types.DataFB, friendID string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	form := url.Values{
		"uid":     {friendID},
		"fb_dtsg": {fb.FbDtsg},
		"jazoest": {fb.Jazoest},
		"__user":  {fb.FacebookID},
		"__a":     {"1"},
		"__req":   {utils.NextRequestBase36()},
		"__rev":   {fb.ClientRevision},
	}

	return executePost(ctx, client, fb, UnfriendURL, form)
}
