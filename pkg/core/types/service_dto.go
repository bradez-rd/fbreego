package types

// CookieInput representa un par de clave/valor de cookie enviado al crear una sesión.
type CookieInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SessionCreateInput representa el payload para registrar una nueva sesión en el servicio.
type SessionCreateInput struct {
	Label    string        `json:"label"`
	TenantID string        `json:"tenantId"`
	Cookies  []CookieInput `json:"cookies"`
}

// TextMessageInput representa la solicitud para enviar un mensaje de texto.
type TextMessageInput struct {
	ThreadID         string `json:"threadId"`
	Text             string `json:"text"`
	ThreadType       string `json:"threadType"` // "user" o "group", default "user"
	ReplyToMessageID string `json:"replyToMessageId,omitempty"`
}

// ReactionInput representa la solicitud para añadir o quitar una reacción emoji a un mensaje.
type ReactionInput struct {
	MessageID string `json:"messageId"`
	Emoji     string `json:"emoji"`
	Action    string `json:"action"` // "add" o "remove", default "add"
}

// UnsendInput representa la solicitud para cancelar el envío de un mensaje.
type UnsendInput struct {
	MessageID string `json:"messageId"`
}

// BlockInput representa la solicitud para bloquear o desbloquear un usuario.
type BlockInput struct {
	UserID string `json:"userId"`
	Action string `json:"action"` // "block" o "unblock", default "block"
}

// MediaUploadInput representa el payload JSON para subir un archivo en base64.
type MediaUploadInput struct {
	FileName      string `json:"fileName"`
	MimeType      string `json:"mimeType,omitempty"`
	ContentBase64 string `json:"contentBase64"`
}

// MediaMessageInput representa el payload para enviar mensajes multimedia previamente cargados.
type MediaMessageInput struct {
	ThreadID   string   `json:"threadId"`
	MediaIDs   []string `json:"mediaIds"`
	ThreadType string   `json:"threadType"` // "user" o "group", default "user"
	Text       string   `json:"text,omitempty"`
}

// WebhookConfigInput representa el payload para actualizar la configuración de webhooks.
type WebhookConfigInput struct {
	Enabled     bool     `json:"enabled"`
	URL         string   `json:"url"`
	Events      []string `json:"events"`
	Secret      *string  `json:"secret,omitempty"`
	ClearSecret bool     `json:"clearSecret"`
}

// WebhookTestInput representa la solicitud de sondeo/test de un webhook.
type WebhookTestInput struct {
	URL    *string `json:"url,omitempty"`
	Secret *string `json:"secret,omitempty"`
}
