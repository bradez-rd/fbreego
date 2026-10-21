package logger

import (
	"fmt"
	"log"
	"os"
	"regexp"
)

var (
	cookieRegex     = regexp.MustCompile(`(?i)\b(c_user|xs|fr|datr|sb|presence|access_token|fb_dtsg)=([^;\s",&]+)`)
	sensitiveKVReg  = regexp.MustCompile(`(?i)("(?:cookie|cookies|access_token|token|secret|password|authorization|fb_dtsg|xs|c_user)"\s*:\s*")[^"]+(")`)
	cookieObjectReg = regexp.MustCompile(`(?i)("name"\s*:\s*"(?:c_user|xs|fr|datr|sb|presence|access_token|fb_dtsg)"[^{}]*"value"\s*:\s*")[^"]+(")`)
	plainSecretReg  = regexp.MustCompile(`(?i)\b(password|secret|token|authorization)\s*([:=])\s*([^\s,;]+)`)
)

// DefaultLogger es el logger global del paquete con soporte de redacción.
var DefaultLogger = log.New(os.Stderr, "[fbreego] ", log.LstdFlags|log.Lmsgprefix)

// RedactSensitive oculta información confidencial (tokens, contraseñas, cookies de sesión).
func RedactSensitive(input string) string {
	res := cookieRegex.ReplaceAllString(input, "$1=<redacted>")
	res = cookieObjectReg.ReplaceAllString(res, "${1}<redacted>${2}")
	res = sensitiveKVReg.ReplaceAllString(res, "${1}<redacted>${2}")
	return plainSecretReg.ReplaceAllString(res, "$1$2 <redacted>")
}

// Info escribe un mensaje informativo aplicando redacción de secretos.
func Info(format string, v ...any) {
	DefaultLogger.Printf("%s", RedactSensitive(fmt.Sprintf(format, v...)))
}

// Error escribe un mensaje de error aplicando redacción de secretos.
func Error(format string, v ...any) {
	DefaultLogger.Printf("[ERROR] %s", RedactSensitive(fmt.Sprintf(format, v...)))
}

// Debug escribe un mensaje de depuración si está habilitado.
func Debug(format string, v ...any) {
	if os.Getenv("FBCHAT_DEBUG") == "1" || os.Getenv("FBCHAT_DEBUG") == "true" {
		DefaultLogger.Printf("[DEBUG] %s", RedactSensitive(fmt.Sprintf(format, v...)))
	}
}
