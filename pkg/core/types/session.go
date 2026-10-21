package types

import (
	"strings"
)

// RequiredSessionFields son los campos obligatorios para considerar válida una sesión de Facebook.
var RequiredSessionFields = []string{
	"fb_dtsg",
	"jazoest",
	"sessionID",
	"FacebookID",
	"clientRevision",
}

// DataFB representa la sesión autenticada extraída de Facebook (equivalente a DataFB en Python).
type DataFB struct {
	FbDtsg         string `json:"fb_dtsg"`
	FbDtsgAg       string `json:"fb_dtsg_ag"`
	Jazoest        string `json:"jazoest"`
	Hash           string `json:"hash"`
	SessionID      string `json:"sessionID"`
	FacebookID     string `json:"FacebookID"`
	ClientRevision string `json:"clientRevision"`
	CookieFacebook string `json:"cookieFacebook"`
}

// HasRequiredFields verifica si todos los campos requeridos de la sesión están presentes y son no vacíos.
func (d *DataFB) HasRequiredFields() (bool, []string) {
	if d == nil {
		return false, RequiredSessionFields
	}

	var missing []string
	if strings.TrimSpace(d.FbDtsg) == "" {
		missing = append(missing, "fb_dtsg")
	}
	if strings.TrimSpace(d.Jazoest) == "" {
		missing = append(missing, "jazoest")
	}
	if strings.TrimSpace(d.SessionID) == "" {
		missing = append(missing, "sessionID")
	}
	if strings.TrimSpace(d.FacebookID) == "" {
		missing = append(missing, "FacebookID")
	}
	if strings.TrimSpace(d.ClientRevision) == "" {
		missing = append(missing, "clientRevision")
	}

	return len(missing) == 0, missing
}

// AttachmentResult representa el resultado de la carga de un archivo multimedia en Facebook.
type AttachmentResult struct {
	AttachmentID       string         `json:"attachmentID"`
	AttachmentURL      string         `json:"attachmentUrl,omitempty"`
	AttachmentType     string         `json:"attachmentType"`
	AttachmentDataSend map[string]any `json:"attachmentDataSend,omitempty"`
}
