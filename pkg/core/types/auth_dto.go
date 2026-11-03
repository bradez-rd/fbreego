package types

import "fmt"

// LoginSuccessPayload representa el resultado de una autenticación exitosa mediante FB4A.
type LoginSuccessPayload struct {
	SetCookies          string           `json:"setCookies"`
	AccessTokenFB       string           `json:"accessTokenFB"`
	CookiesKeyValueList []map[string]any `json:"cookiesKeyValueList,omitempty"`
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

// TwoFactorChallengeError representa el caso donde Facebook requiere completar un desafío 2FA web o Bloks.
type TwoFactorChallengeError struct {
	Code         int    `json:"code"`
	Subcode      int    `json:"subcode"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ChallengeURL string `json:"challengeUrl,omitempty"`
	TOTPCode     string `json:"totpCode,omitempty"`
	FBTraceID    string `json:"fbtraceId,omitempty"`
}

func (e *TwoFactorChallengeError) Error() string {
	if e.ChallengeURL != "" {
		return fmt.Sprintf("facebook 2fa challenge [%d:%d]: %s (url: %s)", e.Code, e.Subcode, e.Description, e.ChallengeURL)
	}
	return fmt.Sprintf("facebook 2fa challenge [%d:%d]: %s", e.Code, e.Subcode, e.Description)
}

// TOTPResponse representa el resultado de la generación de un código TOTP.
type TOTPResponse struct {
	Code             string `json:"code"`
	RemainingSeconds int    `json:"remainingSeconds"`
}
