package types

import "strings"

// CookieInput representa una cookie HTTP con sus atributos de seguridad y vigencia.
type CookieInput struct {
	Name           string  `json:"name"`
	Value          string  `json:"value"`
	Domain         string  `json:"domain,omitempty"`
	Path           string  `json:"path,omitempty"`
	Expires        string  `json:"expires,omitempty"`
	ExpirationDate float64 `json:"expirationDate,omitempty"`
	Secure         bool    `json:"secure,omitempty"`
	HttpOnly       bool    `json:"httpOnly,omitempty"`
	SameSite       string  `json:"sameSite,omitempty"`
}

// SessionCreateInput representa el payload para registrar una nueva sesión en el servicio.
type SessionCreateInput struct {
	Label    string        `json:"label"`
	TenantID string        `json:"tenantId"`
	Cookies  []CookieInput `json:"cookies"`
}

// Constantes de tipos de conversación reconocidos.
const (
	ThreadTypeUser  = "user"
	ThreadTypeGroup = "group"
	ThreadTypeAIBot = "ai_bot" // Meta AI (código numérico 201 en Facebook LightSpeed)
)

// NormalizeThreadType normaliza cadenas o códigos numéricos legados (ej. "201" de Meta AI)
// garantizando que bots y usuarios directos nunca se clasifiquen erróneamente como grupos.
func NormalizeThreadType(threadType string) string {
	switch strings.ToLower(strings.TrimSpace(threadType)) {
	case "group", "2":
		return ThreadTypeGroup
	case "ai_bot", "201", "bot", "ai":
		return ThreadTypeAIBot
	default:
		return ThreadTypeUser
	}
}

// TextMessageInput representa la solicitud para enviar un mensaje de texto.
type TextMessageInput struct {
	ThreadID         string `json:"threadId"`
	Text             string `json:"text"`
	ThreadType       string `json:"threadType"` // "user", "group" o "ai_bot" (201), default "user"
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
