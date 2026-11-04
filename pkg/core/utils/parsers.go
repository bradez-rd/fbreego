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

// ParseGraphQLBatchResponse decodifica una respuesta de GraphQL batch de Facebook, soportando múltiples objetos JSON concatenados (NDJSON / stream).
func ParseGraphQLBatchResponse(raw []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(raw)
	prefix := []byte("for (;;);")
	if bytes.HasPrefix(trimmed, prefix) {
		trimmed = bytes.TrimSpace(trimmed[len(prefix):])
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()

	for decoder.More() {
		var chunk map[string]any
		if err := decoder.Decode(&chunk); err != nil {
			break
		}
		if _, ok := chunk["o0"]; ok {
			return chunk, nil
		}
		if _, ok := chunk["data"]; ok {
			return map[string]any{"o0": chunk}, nil
		}
	}

	// Fallback estándar si viene como un solo objeto JSON
	var fallback map[string]any
	if err := json.Unmarshal(trimmed, &fallback); err == nil {
		return fallback, nil
	}

	return nil, fmt.Errorf("no se encontró el objeto o0 en la respuesta de graphqlbatch")
}
