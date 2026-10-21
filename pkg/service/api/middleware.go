package api

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/bradez-rd/fbreego/pkg/core/types"
)

type authConfiguration struct {
	Required   bool
	Configured bool
	Username   string
	Password   string
}

func envFlag(name string, defaultValue bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return defaultValue
	}
	switch strings.ToLower(value) {
	case "1", "true", "on", "yes":
		return true
	case "0", "false", "off", "no":
		return false
	default:
		return defaultValue
	}
}

// currentAuthConfiguration is the single source of truth for REST and WebSocket auth.
// Authentication is fail-closed by default and must be explicitly disabled for local use.
func currentAuthConfiguration() authConfiguration {
	username := strings.TrimSpace(os.Getenv("FBCHAT_API_USERNAME"))
	password := strings.TrimSpace(os.Getenv("FBCHAT_API_PASSWORD"))
	configured := username != "" && password != ""
	required := envFlag("FBCHAT_API_AUTH_REQUIRED", configured)
	return authConfiguration{
		Required:   required,
		Configured: configured,
		Username:   username,
		Password:   password,
	}
}

// ValidateRuntimeConfiguration verifies security-sensitive startup settings.
func ValidateRuntimeConfiguration() error {
	config := currentAuthConfiguration()
	if config.Required && !config.Configured {
		return fmt.Errorf("FBCHAT_API_USERNAME y FBCHAT_API_PASSWORD son obligatorios cuando la autenticación está habilitada")
	}
	return nil
}

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
		config := currentAuthConfiguration()
		if !config.Required {
			next.ServeHTTP(w, r)
			return
		}

		if !config.Configured {
			JSONError(w, "AUTH_NOT_CONFIGURED", "API authentication is required but credentials are not configured.", http.StatusServiceUnavailable, nil, "")
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(config.Username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(config.Password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="fbchat-v2 Provider"`)
			JSONError(w, "UNAUTHORIZED", "Invalid API credentials.", http.StatusUnauthorized, nil, "")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func allowedCORSOrigins() map[string]struct{} {
	allowed := make(map[string]struct{})
	for _, origin := range strings.Split(os.Getenv("FBCHAT_API_CORS_ORIGINS"), ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return allowed
}

// CORSMiddleware only enables cross-origin access for explicitly configured origins.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		allowed := allowedCORSOrigins()
		_, exactMatch := allowed[origin]
		_, wildcard := allowed["*"]
		originAllowed := origin == "" || exactMatch || wildcard

		if origin != "" && originAllowed {
			if wildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-File-Name")
		}

		if r.Method == http.MethodOptions {
			if !originAllowed {
				JSONError(w, "CORS_ORIGIN_DENIED", "Origin is not allowed.", http.StatusForbidden, nil, "")
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
