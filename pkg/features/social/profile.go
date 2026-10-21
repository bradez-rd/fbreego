package social

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

// ChangeBio actualiza el texto de la biografía pública del perfil en Facebook.
func ChangeBio(ctx context.Context, client *http.Client, fb *types.DataFB, newBio string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	variables := map[string]any{
		"input": map[string]any{
			"bio": newBio,
			"publish_bio_feed_story": false,
			"actor_id": fb.FacebookID,
			"client_mutation_id": utils.NextRequestBase36(),
		},
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "ProfileCometBioMutation", "4529104920491022", true)
	form.Set("variables", string(varBytes))

	return executePost(ctx, client, fb, GraphQLURL, form)
}

// ToggleProfessionalMode activa o desactiva las capacidades del Modo Profesional en la cuenta.
func ToggleProfessionalMode(ctx context.Context, client *http.Client, fb *types.DataFB, enable bool) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	status := "TURN_OFF"
	if enable {
		status = "TURN_ON"
	}

	variables := map[string]any{
		"input": map[string]any{
			"action": status,
			"actor_id": fb.FacebookID,
			"client_mutation_id": utils.NextRequestBase36(),
		},
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "ProfessionalModeMutation", "4920194019284712", true)
	form.Set("variables", string(varBytes))

	return executePost(ctx, client, fb, GraphQLURL, form)
}
