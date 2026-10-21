package utils

import (
	"net/url"

	"github.com/bradez-rd/fbreego/pkg/core/types"
)

// BuildFormAll construye el formulario estándar que Facebook requiere en sus peticiones POST y RelayModern GraphQL.
func BuildFormAll(fb *types.DataFB, friendlyName string, docID string, requireGraphQL bool) url.Values {
	form := url.Values{
		"fb_dtsg":  {fb.FbDtsg},
		"jazoest":  {fb.Jazoest},
		"__a":      {"1"},
		"__user":   {fb.FacebookID},
		"__req":    {NextRequestBase36()},
		"__rev":    {fb.ClientRevision},
		"av":       {fb.FacebookID},
	}

	if requireGraphQL {
		form.Set("fb_api_caller_class", "RelayModern")
		if friendlyName != "" {
			form.Set("fb_api_req_friendly_name", friendlyName)
		}
		form.Set("server_timestamps", "true")
		if docID != "" {
			form.Set("doc_id", docID)
		}
	}

	return form
}
