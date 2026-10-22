package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/bradez-rd/fbreego/pkg/core/contracts"
	"github.com/bradez-rd/fbreego/pkg/core/httputil"
	"github.com/bradez-rd/fbreego/pkg/core/storage"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

var (
	ErrTokensMissing      = errors.New("faltan tokens requeridos en la respuesta de facebook")
	ErrSessionExpired     = httputil.ErrSessionExpired
	ErrAccountRestricted  = httputil.ErrAccountRestricted
	ErrMarkupUnrecognized = errors.New("la estructura HTML de Facebook no contiene los tokens de sesión esperados")
)

// Patrones regex precompilados y resilientes para extraer tokens estructurados de Facebook Comet y LightSpeed.
var (
	// DTSG: busca módulos JS tipo ["DTSGInitialData", [], {"token": "..."}] o DTSGInitData
	dtsgModuleRegex = regexp.MustCompile(`\[\s*"(?:DTSGInitialData|DTSGInitData)"\s*,\s*(?:\[\]|\{\})\s*,\s*\{[^}]*?"token"\s*:\s*"([^"]+)"`)
	dtsgKeyRegex    = regexp.MustCompile(`"(?:DTSGInitialData|DTSGInitData)"[^}]*?"token"\s*:\s*"([^"]+)"`)
	dtsgInputRegex  = regexp.MustCompile(`<input[^>]+name="fb_dtsg"[^>]+value="([^"]+)"|<input[^>]+value="([^"]+)"[^>]+name="fb_dtsg"`)
	asyncTokenRegex = regexp.MustCompile(`"async_get_token"\s*:\s*"([^"]+)"`)

	// ActorID / UserID
	currentUserModuleRegex = regexp.MustCompile(`\[\s*"CurrentUserInitialData"\s*,\s*(?:\[\]|\{\})\s*,\s*\{[^}]*?"(?:USER_ID|ACCOUNT_ID)"\s*:\s*"(\d+)"`)
	actorIDRegex           = regexp.MustCompile(`"(?:actorID|USER_ID|ACCOUNT_ID)"\s*:\s*"(\d+)"`)
	cUserCookieRegex       = regexp.MustCompile(`(?:^|;\s*)c_user=(\d+)`)

	// Jazoest
	jazoestInputRegex = regexp.MustCompile(`<input[^>]+name="jazoest"[^>]+value="(\d+)"|<input[^>]+value="(\d+)"[^>]+name="jazoest"`)
	jazoestKeyRegex   = regexp.MustCompile(`jazoest=(\d+)`)

	// SessionID & ClientRevision
	sessionIDRegex = regexp.MustCompile(`"sessionId"\s*:\s*"([^"]+)"`)
	clientRevRegex = regexp.MustCompile(`"(?:client_revision|clientRevision|rev)"\s*:\s*(\d+)`)

	// Hash
	hashRegex = regexp.MustCompile(`"hash"\s*:\s*"([^"]+)"`)
)

// ComputeJazoest calcula matemáticamente el parámetro jazoest a partir de fb_dtsg según el algoritmo oficial de Facebook.
// Algoritmo: '2' + sum(ASCII(dtsg[i])).
func ComputeJazoest(dtsg string) string {
	dtsg = strings.TrimSpace(dtsg)
	if dtsg == "" {
		return ""
	}
	var sum int
	for _, r := range dtsg {
		sum += int(r)
	}
	return fmt.Sprintf("2%d", sum)
}

// ExtractCookieValue extrae el valor de una cookie por nombre desde una cadena de cookies "key=val; key2=val2".
func ExtractCookieValue(cookieHeader, name string) string {
	parts := strings.Split(cookieHeader, ";")
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if strings.HasPrefix(trimmed, name+"=") {
			return strings.TrimPrefix(trimmed, name+"=")
		}
	}
	return ""
}

// ExtractBetween busca y extrae la subcadena entre start y end dentro de s (mantenida por compatibilidad).
func ExtractBetween(s, start, end string) string {
	idxStart := strings.Index(s, start)
	if idxStart == -1 {
		return ""
	}
	idxStart += len(start)
	idxEnd := strings.Index(s[idxStart:], end)
	if idxEnd == -1 {
		return ""
	}
	return s[idxStart : idxStart+idxEnd]
}

// ParseSessionData analiza el payload HTML/JS devuelto por Facebook y extrae los tokens de sesión
// de forma resiliente, desacoplada de maquetación HTML y con cálculo dinámico de integridad.
func ParseSessionData(html, setCookies string) (*types.DataFB, error) {
	data := &types.DataFB{
		CookieFacebook: setCookies,
	}

	// 1. Extraer FacebookID: Primera prioridad desde la cookie c_user autorizada por el usuario
	if cUser := ExtractCookieValue(setCookies, "c_user"); cUser != "" {
		data.FacebookID = cUser
	}
	if data.FacebookID == "" {
		if m := cUserCookieRegex.FindStringSubmatch(html); len(m) > 1 {
			data.FacebookID = m[1]
		}
	}
	if data.FacebookID == "" {
		if m := currentUserModuleRegex.FindStringSubmatch(html); len(m) > 1 {
			data.FacebookID = m[1]
		}
	}
	if data.FacebookID == "" {
		if m := actorIDRegex.FindStringSubmatch(html); len(m) > 1 {
			data.FacebookID = m[1]
		}
	}
	if data.FacebookID == "" {
		data.FacebookID = ExtractBetween(html, `"actorID":"`, `"`)
	}
	if data.FacebookID == "" {
		data.FacebookID = ExtractBetween(html, `"USER_ID":"`, `"`)
	}

	// Validar que FacebookID sea estrictamente numérico si se extrajo algo
	if data.FacebookID != "" {
		for _, r := range data.FacebookID {
			if !unicode.IsDigit(r) {
				return nil, fmt.Errorf("FacebookID no numérico detectado: %s", data.FacebookID)
			}
		}
	}

	// 2. Extraer fb_dtsg (Token CSRF de Facebook)
	if m := dtsgModuleRegex.FindStringSubmatch(html); len(m) > 1 {
		data.FbDtsg = m[1]
	}
	if data.FbDtsg == "" {
		if m := dtsgKeyRegex.FindStringSubmatch(html); len(m) > 1 {
			data.FbDtsg = m[1]
		}
	}
	if data.FbDtsg == "" {
		if m := dtsgInputRegex.FindStringSubmatch(html); len(m) > 1 {
			if m[1] != "" {
				data.FbDtsg = m[1]
			} else if len(m) > 2 && m[2] != "" {
				data.FbDtsg = m[2]
			}
		}
	}
	if data.FbDtsg == "" {
		data.FbDtsg = ExtractBetween(html, `DTSGInitialData",[],{"token":"`, `"`)
	}

	// 3. Extraer FbDtsgAg (async_get_token)
	if m := asyncTokenRegex.FindStringSubmatch(html); len(m) > 1 {
		data.FbDtsgAg = m[1]
	}
	if data.FbDtsgAg == "" {
		data.FbDtsgAg = ExtractBetween(html, `async_get_token":"`, `"`)
	}

	// 4. Extraer o calcular jazoest
	if m := jazoestInputRegex.FindStringSubmatch(html); len(m) > 1 {
		if m[1] != "" {
			data.Jazoest = m[1]
		} else if len(m) > 2 && m[2] != "" {
			data.Jazoest = m[2]
		}
	}
	if data.Jazoest == "" {
		if m := jazoestKeyRegex.FindStringSubmatch(html); len(m) > 1 {
			data.Jazoest = m[1]
		}
	}
	if data.Jazoest == "" {
		data.Jazoest = ExtractBetween(html, `name="jazoest" value="`, `"`)
	}
	if data.Jazoest == "" {
		data.Jazoest = ExtractBetween(html, `jazoest=`, `"`)
	}
	// Fallback algorítmico robusto: si no está en el HTML, calcular jazoest a partir de fb_dtsg
	if data.Jazoest == "" && data.FbDtsg != "" {
		data.Jazoest = ComputeJazoest(data.FbDtsg)
	}

	// 5. Extraer Hash
	if m := hashRegex.FindStringSubmatch(html); len(m) > 1 {
		data.Hash = m[1]
	}
	if data.Hash == "" {
		data.Hash = ExtractBetween(html, `hash":"`, `"`)
	}

	// 6. Extraer SessionID
	if m := sessionIDRegex.FindStringSubmatch(html); len(m) > 1 {
		data.SessionID = m[1]
	}
	if data.SessionID == "" {
		data.SessionID = ExtractBetween(html, `sessionId":"`, `"`)
	}
	// Fallback: Si Facebook no incluye sessionId literal en el HTML, generar un identificador de época seguro
	if data.SessionID == "" && data.FbDtsg != "" {
		data.SessionID = fmt.Sprintf("%d", utils.GenerateSessionID())
	}

	// 7. Extraer ClientRevision (__rev)
	if m := clientRevRegex.FindStringSubmatch(html); len(m) > 1 {
		data.ClientRevision = m[1]
	}
	if data.ClientRevision == "" {
		data.ClientRevision = ExtractBetween(html, `client_revision":`, `,`)
	}
	// Fallback razonable si el resto de los tokens de sesión son válidos
	if data.ClientRevision == "" && data.FbDtsg != "" {
		data.ClientRevision = "101827412"
	}

	// Validación estricta de campos requeridos
	ok, missing := data.HasRequiredFields()
	if !ok {
		// Discriminar si la respuesta es una página de login o checkpoint
		lower := strings.ToLower(html)
		if strings.Contains(lower, `login_form`) || strings.Contains(lower, `id="login"`) || strings.Contains(lower, `checkpoint`) {
			return nil, fmt.Errorf("%w: facebook solicitó inicio de sesión o checkpoint", ErrSessionExpired)
		}
		return nil, fmt.Errorf("%w: %s", ErrTokensMissing, strings.Join(missing, ", "))
	}

	return data, nil
}

// ParseHomeResponse mantiene compatibilidad hacia atrás delegando en ParseSessionData.
func ParseHomeResponse(html, setCookies string) (*types.DataFB, error) {
	return ParseSessionData(html, setCookies)
}

// ExtractSession realiza una petición HTTP GET a Facebook usando las cookies provistas y extrae la sesión.
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

	baseURL := contracts.DefaultRegistry().GetEndpoint("base")
	if baseURL == "" {
		baseURL = contracts.DefaultFacebookBaseURL
	}

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("error al construir petición a facebook: %w", err)
	}

	// Aplicar cabeceras coherentes con User-Agent y Client Hints estables
	utils.ApplyHeaders(req, "www.facebook.com")
	req.Header.Set("Cookie", setCookies)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
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

	if err := httputil.CheckHTTPResponse(resp); err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error al leer cuerpo html de facebook: %w", err)
	}

	return ParseSessionData(string(bodyBytes), setCookies)
}
