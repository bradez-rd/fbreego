package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/bradez-rd/fbreego/pkg/core/types"
)

// JSONResponse escribe una respuesta JSON estructurada con el código HTTP correspondiente.
func JSONResponse(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

// JSONSuccess escribe una respuesta de éxito estándar: {"code": "SUCCESS", "message": "...", "results": ...}
func JSONSuccess(w http.ResponseWriter, message string, results any, statusCode int) {
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	JSONResponse(w, statusCode, types.ServiceResponse{
		Code:    "SUCCESS",
		Message: message,
		Results: results,
	})
}

// JSONError escribe una respuesta de error estándar.
func JSONError(w http.ResponseWriter, code, message string, statusCode int, details any, traceID string) {
	JSONResponse(w, statusCode, types.ServiceErrorResponse{
		Code:    code,
		Message: message,
		Details: details,
		TraceID: traceID,
	})
}

// BasicAuthMiddleware verifica las credenciales de la API si la autenticación está habilitada.
func BasicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqAuth := os.Getenv("FBCHAT_API_AUTH_REQUIRED")
		// Por defecto requerida a menos que sea explícitamente "0" o "false"
		if reqAuth == "0" || strings.EqualFold(reqAuth, "false") || strings.EqualFold(reqAuth, "off") {
			next.ServeHTTP(w, r)
			return
		}

		expectedUser := os.Getenv("FBCHAT_API_USERNAME")
		expectedPass := os.Getenv("FBCHAT_API_PASSWORD")

		// Si es requerida pero no está configurada, devolver 503
		if strings.TrimSpace(expectedUser) == "" || strings.TrimSpace(expectedPass) == "" {
			JSONError(w, "AUTH_NOT_CONFIGURED", "API authentication is required but not configured.", http.StatusServiceUnavailable, nil, "")
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(expectedUser)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(expectedPass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="fbchat-v2 Provider"`)
			JSONError(w, "UNAUTHORIZED", "Invalid API credentials.", http.StatusUnauthorized, nil, "")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware añade cabeceras para permitir orígenes cruzados en el portal web y clientes externos.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
