package session

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/bradez-rd/fbreego/pkg/core/storage"
	"github.com/bradez-rd/fbreego/pkg/core/types"
)

// ExtractBetween busca y extrae la subcadena entre start y end dentro de html.
func ExtractBetween(html, start, end string) string {
	idxStart := strings.Index(html, start)
	if idxStart == -1 {
		return ""
	}
	idxStart += len(start)
	idxEnd := strings.Index(html[idxStart:], end)
	if idxEnd == -1 {
		return ""
	}
	return html[idxStart : idxStart+idxEnd]
}

// ParseHomeResponse analiza el contenido HTML devuelto por la portada de Facebook y extrae los tokens de sesión.
func ParseHomeResponse(html, setCookies string) (*types.DataFB, error) {
	data := &types.DataFB{
		FbDtsg:         ExtractBetween(html, `DTSGInitialData",[],{"token":"`, `"`),
		FbDtsgAg:       ExtractBetween(html, `async_get_token":"`, `"`),
		Jazoest:        ExtractBetween(html, `jazoest=`, `"`),
		Hash:           ExtractBetween(html, `hash":"`, `"`),
		SessionID:      ExtractBetween(html, `sessionId":"`, `"`),
		FacebookID:     ExtractBetween(html, `"actorID":"`, `"`),
		ClientRevision: ExtractBetween(html, `client_revision":`, `,`),
		CookieFacebook: setCookies,
	}

	// Si no encuentra jazoest en URL o atributo directo, intentar en input de formulario
	if data.Jazoest == "" {
		data.Jazoest = ExtractBetween(html, `name="jazoest" value="`, `"`)
	}

	// Si no encuentra actorID con comillas dobles, intentar búsqueda alternativa de USER_ID
	if data.FacebookID == "" {
		data.FacebookID = ExtractBetween(html, `"USER_ID":"`, `"`)
	}
	if data.FacebookID == "" {
		data.FacebookID = ExtractBetween(html, `c_user=`, `;`)
	}

	ok, missing := data.HasRequiredFields()
	if !ok {
		return nil, fmt.Errorf("faltan tokens requeridos en la respuesta de facebook: %s", strings.Join(missing, ", "))
	}

	// Validar que FacebookID sea completamente numérico
	for _, r := range data.FacebookID {
		if !unicode.IsDigit(r) {
			return nil, fmt.Errorf("FacebookID no numérico detectado: %s", data.FacebookID)
		}
	}

	return data, nil
}

// ExtractSession realiza una petición HTTP GET a la página principal de Facebook usando las cookies provistas y extrae la sesión.
func ExtractSession(ctx context.Context, client *http.Client, setCookies string, store storage.SessionStorage) (*types.DataFB, error) {
	if strings.TrimSpace(setCookies) == "" && store != nil {
		loaded, err := store.Load()
		if err == nil {
			setCookies = loaded
		}
	}

	setCookies = strings.TrimSpace(setCookies)
	if setCookies == "" {
		return nil, fmt.Errorf("no se proporcionaron cookies para inicializar la sesión")
	}

	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.facebook.com/", nil)
	if err != nil {
		return nil, fmt.Errorf("error al construir petición a facebook: %w", err)
	}

	req.Header.Set("Authority", "www.facebook.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cookie", setCookies)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fallo de red al consultar homepage de facebook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("error http de facebook al obtener homepage: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error al leer cuerpo html de facebook: %w", err)
	}

	return ParseHomeResponse(string(bodyBytes), setCookies)
}
