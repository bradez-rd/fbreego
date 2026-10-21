package reactions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

const GraphQLURL = "https://www.facebook.com/api/graphql/"
const ReactionDocID = "2972740269476564"

// ReactMessage añade o retira una reacción emoji a un mensaje de Messenger.
func ReactMessage(ctx context.Context, client *http.Client, fb *types.DataFB, messageID, emoji, action string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	reactionAction := "ADD_REACTION"
	if strings.ToLower(action) == "remove" {
		reactionAction = "REMOVE_REACTION"
	}

	variables := map[string]any{
		"data": map[string]any{
			"client_mutation_id": utils.NextRequestBase36(),
			"actor_id":           fb.FacebookID,
			"action":             reactionAction,
			"message_id":         messageID,
			"reaction":           emoji,
		},
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "useFXIMUpdateUserPostMutation", ReactionDocID, true)
	form.Set("variables", string(varBytes))

	req, err := http.NewRequestWithContext(ctx, "POST", GraphQLURL, strings.NewReader(form.Encode()))
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
		return nil, fmt.Errorf("error de red al reaccionar al mensaje: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return utils.ParseFacebookJSON(bodyBytes, true)
}
