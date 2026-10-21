package logger

import (
	"log"
	"os"
	"regexp"
)

var (
	cookieRegex    = regexp.MustCompile(`(?i)\b(c_user|xs|fr|datr|sb|presence|access_token)=([^;\s",]+)`)
	sensitiveKVReg = regexp.MustCompile(`(?i)("(?:cookie|access_token|token|secret|password|authorization)":\s*")[^"]+(")`)
)

// DefaultLogger es el logger global del paquete con soporte de redacción.
var DefaultLogger = log.New(os.Stderr, "[fbreego] ", log.LstdFlags|log.Lmsgprefix)

// RedactSensitive oculta información confidencial (tokens, contraseñas, cookies de sesión).
func RedactSensitive(input string) string {
	res := cookieRegex.ReplaceAllString(input, "$1=<redacted>")
	res = sensitiveKVReg.ReplaceAllString(res, "${1}<redacted>${2}")
	return res
}

// Info escribe un mensaje informativo aplicando redacción de secretos.
func Info(format string, v ...any) {
	DefaultLogger.Printf(RedactSensitive(format), v...)
}

// Error escribe un mensaje de error aplicando redacción de secretos.
func Error(format string, v ...any) {
	DefaultLogger.Printf("[ERROR] "+RedactSensitive(format), v...)
}

// Debug escribe un mensaje de depuración si está habilitado.
func Debug(format string, v ...any) {
	if os.Getenv("FBCHAT_DEBUG") == "1" || os.Getenv("FBCHAT_DEBUG") == "true" {
		DefaultLogger.Printf("[DEBUG] "+RedactSensitive(format), v...)
	}
}
