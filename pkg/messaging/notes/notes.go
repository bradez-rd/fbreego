package notes

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

const (
	GraphQLURL        = "https://www.facebook.com/api/graphql/"
	CreateNoteDocID   = "6783842415012542"
	DeleteNoteDocID   = "6639209149480112"
	QueryNotesDocID   = "6518721244874001"
)

// CreateNote publica una nota de 24 horas en el perfil de Messenger.
func CreateNote(ctx context.Context, client *http.Client, fb *types.DataFB, text string, audience string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if audience == "" {
		audience = "FRIENDS"
	}

	variables := map[string]any{
		"input": map[string]any{
			"client_mutation_id": utils.NextRequestBase36(),
			"actor_id":           fb.FacebookID,
			"text":               text,
			"audience":           audience,
		},
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "MessengerNotesCreateMutation", CreateNoteDocID, true)
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
		return nil, fmt.Errorf("error de red al crear nota: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return utils.ParseFacebookJSON(bodyBytes, true)
}

// DeleteNote elimina una nota de 24 horas activa de Messenger.
func DeleteNote(ctx context.Context, client *http.Client, fb *types.DataFB, noteID string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	variables := map[string]any{
		"input": map[string]any{
			"client_mutation_id": utils.NextRequestBase36(),
			"actor_id":           fb.FacebookID,
			"note_id":            noteID,
		},
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "MessengerNotesDeleteMutation", DeleteNoteDocID, true)
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
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return utils.ParseFacebookJSON(bodyBytes, true)
}
