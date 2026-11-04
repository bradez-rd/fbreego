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

	coreconfig "github.com/bradez-rd/fbreego/pkg/core/config"
	"github.com/bradez-rd/fbreego/pkg/core/types"
)

type authConfiguration struct {
	Required   bool
	Configured bool
	Username   string
	Password   string
}

func envFlag(name string, defaultValue bool) bool {
	val := os.Getenv(name)
	if val == "" {
		return coreconfig.Bool(name, defaultValue)
	}
	switch strings.ToLower(strings.TrimSpace(val)) {
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
	username := coreconfig.Value("FBREEGO_API_USERNAME")
	if username == "" {
		username = strings.TrimSpace(os.Getenv("FBCHAT_API_USERNAME"))
	}
	password := coreconfig.Value("FBREEGO_API_PASSWORD")
	if password == "" {
		password = strings.TrimSpace(os.Getenv("FBCHAT_API_PASSWORD"))
	}
	configured := username != "" && password != ""
	
	required := true
	if v := os.Getenv("FBCHAT_API_AUTH_REQUIRED"); v != "" {
		required = envFlag("FBCHAT_API_AUTH_REQUIRED", true)
	}
	if v := os.Getenv("FBREEGO_API_AUTH_REQUIRED"); v != "" {
		required = envFlag("FBREEGO_API_AUTH_REQUIRED", required)
	}

	return authConfiguration{
		Required:   required,
		Configured: configured,
		Username:   username,
		Password:   password,
	}
}

// ValidateRuntimeConfiguration verifies security-sensitive startup settings.
func ValidateRuntimeConfiguration() error {
	secret := coreconfig.Value("FBREEGO_SESSION_SECRET")
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("FBCHAT_SESSION_SECRET"))
	}
	if secret != "" {
		if len(secret) < 32 {
			return fmt.Errorf("FBREEGO_SESSION_SECRET debe contener al menos 32 caracteres criptogr\u00e1ficamente seguros")
		}
		secretLower := strings.ToLower(secret)
		if secretLower == "fbchat-default-development-secret-key-change-in-production" ||
			strings.Contains(secretLower, "change-in-production") ||
			strings.Contains(secretLower, "change-me") {
			return fmt.Errorf("FBREEGO_SESSION_SECRET utiliza una clave predeterminada o insegura conocida; configure una clave generada aleatoriamente")
		}
	}

	config := currentAuthConfiguration()
	if config.Required && !config.Configured {
		return fmt.Errorf("FBREEGO_API_USERNAME y FBREEGO_API_PASSWORD son obligatorios cuando la autenticaci\u00f3n est\u00e1 habilitada")
	}
	return nil
}

// JSONResponse serializa e imprime un objeto como respuesta JSON.
func JSONResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

// JSONSuccess formatea y emite una respuesta exitosa estandarizada de servicio.
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

// JSONError formatea y emite una respuesta de error estandarizada de servicio.
func JSONError(w http.ResponseWriter, code, message string, statusCode int, details any, requestID string) {
	JSONResponse(w, statusCode, types.ServiceErrorResponse{
		Code:    code,
		Message: message,
		Details: details,
		TraceID: requestID,
	})
}

// DefaultMaxBodyBytes define el l\u00edmite global por defecto para el tama\u00f1o del cuerpo de peticiones JSON (1 MiB).
const DefaultMaxBodyBytes = 1 << 20

// DecodeJSONBody decodifica el cuerpo de la petici\u00f3n con l\u00edmite de bytes, rechaza campos desconocidos
// y valida que exista un \u00fanico documento JSON para prevenir ataques de agotamiento de memoria o inyecci\u00f3n.
func DecodeJSONBody[T any](w http.ResponseWriter, r *http.Request, maxBytes int64) (T, error) {
	var dst T
	if r.Body == nil {
		return dst, fmt.Errorf("cuerpo de petici\u00f3n vac\u00edo")
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
			return dst, fmt.Errorf("el cuerpo contiene datos JSON adicionales despu\u00e9s del objeto principal")
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

// BasicAuthMiddleware verifica las credenciales de la API si la autenticaci\u00f3n est\u00e1 habilitada,
// y aplica l\u00edmite de tasa ante intentos fallidos consecutivos.
func BasicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config := currentAuthConfiguration()
		if !config.Required {
			next.ServeHTTP(w, r)
			return
		}

		if !config.Configured {
			JSONError(w, "AUTH_NOT_CONFIGURED", "API credentials not configured on server.", http.StatusServiceUnavailable, nil, "")
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
			w.Header().Set("WWW-Authenticate", `Basic realm="fbreego"`)
			JSONError(w, "UNAUTHORIZED", "Invalid API credentials.", http.StatusUnauthorized, nil, "")
			return
		}

		globalAuthLimiter.recordSuccess(ip)
		next.ServeHTTP(w, r)
	})
}

func allowedCORSOrigins() map[string]struct{} {
	allowed := make(map[string]struct{})
	val := coreconfig.Value("FBREEGO_API_CORS_ORIGINS")
	if val == "" {
		val = os.Getenv("FBCHAT_API_CORS_ORIGINS")
	}
	for _, origin := range strings.Split(val, ",") {
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

		// Immediately reject requests with disallowed Origin before reaching any handler.
		if origin != "" && !originAllowed {
			JSONError(w, "CORS_ORIGIN_DENIED", "Origin is not allowed.", http.StatusForbidden, nil, "")
			return
		}

		if origin != "" && originAllowed {
			if wildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
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
