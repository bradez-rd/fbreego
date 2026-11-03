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
	Cookies  []CookieInput `json:"cookies,omitempty"`
	Email    string        `json:"email,omitempty"`
	Password string        `json:"password,omitempty"`
	TOTPKey  string        `json:"totpKey,omitempty"`
}

// LoginCredentialsInput representa los datos para autenticar y registrar una sesión mediante correo y contraseña (+ 2FA opcional).
type LoginCredentialsInput struct {
	Label    string `json:"label,omitempty"`
	TenantID string `json:"tenantId,omitempty"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password"`
	TOTPKey  string `json:"totpKey,omitempty"`
}

// CredentialAuthInput representa la solicitud de login tradicional con usuario/correo y contraseña para fbreego.
type CredentialAuthInput struct {
	Label    string `json:"label,omitempty"`
	TenantID string `json:"tenantId,omitempty"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password"`
	TOTPKey  string `json:"totpKey,omitempty"`
}

// TwoFactorAuthInput representa la solicitud para completar la verificación 2FA con el desafío pendiente.
type TwoFactorAuthInput struct {
	ChallengeID string `json:"challengeId"`
	Code        string `json:"code"`
}

// TOTPGenerateInput representa la solicitud para generar un código 2FA a partir de una clave secreta.
type TOTPGenerateInput struct {
	Key string `json:"key"`
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
