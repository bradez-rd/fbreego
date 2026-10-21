package types

import "fmt"

// LoginSuccessPayload representa el resultado de una autenticación exitosa mediante FB4A.
type LoginSuccessPayload struct {
	SetCookies          string           `json:"setCookies"`
	AccessTokenFB       string           `json:"accessTokenFB"`
	CookiesKeyValueList []map[string]any `json:"cookiesKey-ValueList,omitempty"`
}

// LoginErrorPayload representa los datos de error devueltos por el endpoint de login de Facebook.
type LoginErrorPayload struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	ErrorSubcode *int   `json:"error_subcode,omitempty"`
	ErrorCode    *int   `json:"error_code,omitempty"`
	FBTraceID    string `json:"fbtrace_id,omitempty"`
}

// FacebookAPIError implementa la interfaz error para fallos específicos de la API de Facebook.
type FacebookAPIError struct {
	Code        int    `json:"code"`
	Subcode     int    `json:"subcode,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description"`
	FBTraceID   string `json:"fbtrace_id,omitempty"`
	HTTPStatus  int    `json:"http_status,omitempty"`
}

func (e *FacebookAPIError) Error() string {
	if e.Subcode != 0 {
		return fmt.Sprintf("facebook api error [%d:%d]: %s - %s", e.Code, e.Subcode, e.Title, e.Description)
	}
	return fmt.Sprintf("facebook api error [%d]: %s - %s", e.Code, e.Title, e.Description)
}
