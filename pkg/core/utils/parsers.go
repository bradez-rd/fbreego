package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var htmlTagRegex = regexp.MustCompile(`<[^>]+>`)

// ParseCookieString convierte una cabecera de cookies en un mapa de pares clave/valor.
func ParseCookieString(cookieStr string) map[string]string {
	out := make(map[string]string)
	parts := strings.Split(cookieStr, ";")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || !strings.Contains(p, "=") {
			continue
		}
		k, v, found := strings.Cut(p, "=")
		if found {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

// ClearHTML remueve todas las etiquetas HTML del texto dado.
func ClearHTML(text string) string {
	return htmlTagRegex.ReplaceAllString(text, "")
}

// ParseFacebookJSON decodifica una respuesta JSON de Facebook, suprimiendo opcionalmente el prefijo anti-hijacking `for (;;);`.
func ParseFacebookJSON(raw []byte, stripForLoopPrefix bool) (map[string]any, error) {
	trimmed := bytes.TrimSpace(raw)
	if stripForLoopPrefix {
		prefix := []byte("for (;;);")
		if bytes.HasPrefix(trimmed, prefix) {
			trimmed = bytes.TrimSpace(trimmed[len(prefix):])
		}
	}

	var res map[string]any
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber() // Para preservar enteros grandes sin pérdida de precisión de punto flotante
	if err := decoder.Decode(&res); err != nil {
		return nil, fmt.Errorf("error al decodificar respuesta JSON de facebook: %w (la respuesta no tiene formato JSON válido)", err)
	}

	return res, nil
}
