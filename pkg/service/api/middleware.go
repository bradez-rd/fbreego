package api

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

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
	required := envFlag("FBCHAT_API_AUTH_REQUIRED", true)
	return authConfiguration{
		Required:   required,
		Configured: configured,
		Username:   username,
		Password:   password,
	}
}

// ValidateRuntimeConfiguration verifies security-sensitive startup settings.
func ValidateRuntimeConfiguration() error {
	secret := strings.TrimSpace(os.Getenv("FBCHAT_SESSION_SECRET"))
	if secret == "" {
		return fmt.Errorf("FBCHAT_SESSION_SECRET es obligatorio para el cifrado de sesiones y webhook")
	}
	if len(secret) < 32 {
		return fmt.Errorf("FBCHAT_SESSION_SECRET debe contener al menos 32 caracteres criptográficamente seguros")
	}
	secretLower := strings.ToLower(secret)
	if secretLower == "fbchat-default-development-secret-key-change-in-production" ||
		strings.Contains(secretLower, "change-in-production") ||
		strings.Contains(secretLower, "change-me") {
		return fmt.Errorf("FBCHAT_SESSION_SECRET utiliza una clave predeterminada o insegura conocida; configure una clave generada aleatoriamente")
	}

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

// DefaultMaxBodyBytes define el límite global por defecto para el tamaño del cuerpo de peticiones JSON (1 MiB).
const DefaultMaxBodyBytes = 1 << 20

// DecodeJSONBody decodifica el cuerpo de la petición con límite de bytes, rechaza campos desconocidos
// y valida que exista un único documento JSON para prevenir ataques de agotamiento de memoria o inyección.
func DecodeJSONBody[T any](w http.ResponseWriter, r *http.Request, maxBytes int64) (T, error) {
	var dst T
	if r.Body == nil {
		return dst, fmt.Errorf("cuerpo de petición vacío")
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&dst); err != nil {
		return dst, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return dst, fmt.Errorf("el cuerpo contiene datos JSON adicionales después del objeto principal")
		}
	}
	return dst, nil
}

// authRateLimiter previene ataques de fuerza bruta sobre Basic Auth limitando intentos fallidos por IP.
type authRateLimiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
}

var globalAuthLimiter = &authRateLimiter{
	failures: make(map[string][]time.Time),
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (l *authRateLimiter) check(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-60 * time.Second)

	times := l.failures[ip]
	valid := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	l.failures[ip] = valid
	return len(valid) < 5
}

func (l *authRateLimiter) recordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[ip] = append(l.failures[ip], time.Now())
}

func (l *authRateLimiter) recordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, ip)
}

// BasicAuthMiddleware verifica las credenciales de la API si la autenticación está habilitada,
// y aplica límite de tasa ante intentos fallidos consecutivos.
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

		ip := clientIP(r)
		if !globalAuthLimiter.check(ip) {
			JSONError(w, "AUTH_RATE_LIMITED", "Too many failed authentication attempts. Please try again later.", http.StatusTooManyRequests, nil, "")
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(config.Username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(config.Password)) != 1 {
			globalAuthLimiter.recordFailure(ip)
			w.Header().Set("WWW-Authenticate", `Basic realm="fbchat-v2 Provider"`)
			JSONError(w, "UNAUTHORIZED", "Invalid API credentials.", http.StatusUnauthorized, nil, "")
			return
		}

		globalAuthLimiter.recordSuccess(ip)
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

// CORSMiddleware enables cross-origin access only for explicitly configured origins
// and rejects unauthorized cross-origin requests on all methods (OPTIONS, GET, POST, DELETE, etc.).
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		allowed := allowedCORSOrigins()
		_, exactMatch := allowed[origin]
		_, wildcard := allowed["*"]
		originAllowed := origin == "" || exactMatch || wildcard

		// F-10 fix: Immediately reject requests with disallowed Origin before reaching any handler.
		if origin != "" && !originAllowed {
			JSONError(w, "CORS_ORIGIN_DENIED", "Origin is not allowed.", http.StatusForbidden, nil, "")
			return
		}

		if origin != "" && originAllowed {
			if wildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-File-Name, X-Tenant-ID")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
