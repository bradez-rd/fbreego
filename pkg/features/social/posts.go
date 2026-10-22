package social

import (
	"context"
	"encoding/json"
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

// GraphQLURL y CreatePostURL obtienen las URLs canónicas desde el registro de contratos.
var (
	GraphQLURL   = contracts.GraphQLURL()
	CreatePostURL = contracts.GraphQLURL()
)

// CreatePost publica un nuevo mensaje en el muro personal del usuario en Facebook.
func CreatePost(ctx context.Context, client *http.Client, fb *types.DataFB, text string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	variables := map[string]any{
		"input": map[string]any{
			"composer_entry_point": "inline_composer",
			"composer_source_surface": "timeline",
			"idempotence_token": utils.GenerateClientID(),
			"source": "WWW",
			"message": map[string]any{
				"text": text,
			},
			"actor_id": fb.FacebookID,
			"client_mutation_id": utils.NextRequestBase36(),
		},
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "FeedStoryCreateMutation", "5512398412170321", true)
	form.Set("variables", string(varBytes))

	return executePost(ctx, client, fb, GraphQLURL, form)
}

// DeletePost elimina una publicación del muro de Facebook.
func DeletePost(ctx context.Context, client *http.Client, fb *types.DataFB, postID string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	variables := map[string]any{
		"input": map[string]any{
			"story_id": postID,
			"actor_id": fb.FacebookID,
			"client_mutation_id": utils.NextRequestBase36(),
		},
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "FeedStoryDeleteMutation", "5329104820491021", true)
	form.Set("variables", string(varBytes))

	return executePost(ctx, client, fb, GraphQLURL, form)
}

// ArchivePost traslada una publicación del muro al archivo personal.
func ArchivePost(ctx context.Context, client *http.Client, fb *types.DataFB, postID string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	variables := map[string]any{
		"input": map[string]any{
			"story_id": postID,
			"actor_id": fb.FacebookID,
			"client_mutation_id": utils.NextRequestBase36(),
		},
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	docID := contracts.DocID("FeedStoryArchiveMutation")
	if docID == "" {
		docID = contracts.DefaultDocIDFeedStoryArchive
	}
	form := utils.BuildFormAll(fb, "FeedStoryArchiveMutation", docID, true)
	form.Set("variables", string(varBytes))

	return executePost(ctx, client, fb, contracts.GraphQLURL(), form)
}

// ReactPost reacciona a una publicación de Facebook (LIKE, LOVE, CARE, HAHA, WOW, SAD, ANGRY).
func ReactPost(ctx context.Context, client *http.Client, fb *types.DataFB, feedbackID, reaction string, isUndo bool) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	docID := "4589210492847120"
	friendlyName := "UFI2FeedbackReactMutation"
	if isUndo {
		docID = "4589210492847121"
		friendlyName = "UFI2FeedbackUnreactMutation"
	}

	variables := map[string]any{
		"input": map[string]any{
			"feedback_id": feedbackID,
			"feedback_reaction": reaction,
			"actor_id": fb.FacebookID,
			"client_mutation_id": utils.NextRequestBase36(),
		},
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, friendlyName, docID, true)
	form.Set("variables", string(varBytes))

	return executePost(ctx, client, fb, GraphQLURL, form)
}

func executePost(ctx context.Context, client *http.Client, fb *types.DataFB, endpoint string, form url.Values) (map[string]any, error) {
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
		return nil, fmt.Errorf("error de red en mutación social: %w", err)
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
