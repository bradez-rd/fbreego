package types

// SuccessResponse representa la respuesta estándar de éxito en fbchat_v2: {"success": 1, ...}
type SuccessResponse struct {
	Success  int            `json:"success"` // siempre 1
	Payload  map[string]any `json:"payload,omitempty"`
	Messages string         `json:"messages,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

// ErrorResponse representa la respuesta estándar de error en fbchat_v2: {"error": 1, ...}
type ErrorResponse struct {
	Error    int            `json:"error"` // siempre 1
	Payload  map[string]any `json:"payload,omitempty"`
	Messages string         `json:"messages,omitempty"`
}

// ErrorDetail describe el detalle de validación de un campo en solicitudes REST.
type ErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ServiceResponse es la estructura estándar devuelta por los endpoints REST de fbchat-provider.
type ServiceResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Results any    `json:"results,omitempty"`
}

// ServiceErrorResponse es la estructura estándar devuelta en caso de error HTTP por el microservicio.
type ServiceErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
	TraceID string `json:"traceId,omitempty"`
}
