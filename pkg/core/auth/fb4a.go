package auth

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/types"
)

const (
	FBAuthURL                 = "https://b-graph.facebook.com/auth/login"
	DefaultFB4AAPIKey         = "882a8490361da98702bf97a021ddc14d"
	DefaultFB4AAppAccessToken = "350685531728|62f8ce9f74b12f84c123cc23437a4a32"
	LegacyFB4AUserAgent       = "Dalvik/2.1.0 (Linux; U; Android 7.1.2; SM-G988N Build/NRD90M) [FBAN/FB4A;FBAV/340.0.0.27.113;FBPN/com.facebook.katana;FBLC/en_US;FBBV/324485361;FBCR/Carrier;FBMF/samsung;FBBD/samsung;FBDV/SM-G988N;FBSV/7.1.2;FBCA/x86:armeabi-v7a;FBDM/{density=1.0,width=540,height=960};FB_FW/1;FBRV/0;]"
)

// ErrFB4ALoginDisabled indica que la autenticación mediante emulación FB4A está deshabilitada por razones de seguridad y cumplimiento.
var ErrFB4ALoginDisabled = fmt.Errorf("la emulación de login FB4A está deshabilitada por seguridad y cumplimiento normativo (F-03)")

func FB4ALoginEnabled() bool {
	return os.Getenv("FBCHAT_ENABLE_UNOFFICIAL_FB4A") == "1"
}

var twoFactorSubcodes = map[int]bool{
	1348162: true,
	1348023: true,
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}
	return io.ReadAll(reader)
}

// randomUUID genera un UUID v4 pseudoaleatorio para los campos de dispositivo de FB4A.
func randomUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func randomAlphaNum(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

func fb4aHeaders() http.Header {
	return http.Header{
		"Host":                    {"b-graph.facebook.com"},
		"User-Agent":              {LegacyFB4AUserAgent},
		"Content-Type":            {"application/x-www-form-urlencoded"},
		"Accept-Language":         {"es-ES,es;q=0.9,en-US;q=0.8,en;q=0.7"},
		"X-Fb-Connection-Type":    {"unknown"},
		"X-Fb-Connection-Quality": {"EXCELLENT"},
		"Authorization":           {"OAuth null"},
		"X-Fb-Friendly-Name":      {"authenticate"},
		"X-Fb-Server-Cluster":     {"True"},
	}
}

func buildBaseForm(username, password, credentialsType, tryNum, deviceID, adid, machineID string) url.Values {
	jazoest := "22421"
	if credentialsType == "two_factor" {
		jazoest = "22327"
	}
	form := url.Values{
		"adid":                             {adid},
		"advertiser_id":                    {adid},
		"format":                           {"json"},
		"device_id":                        {deviceID},
		"family_device_id":                 {deviceID},
		"secure_family_device_id":          {deviceID},
		"machine_id":                       {machineID},
		"email":                            {username},
		"password":                         {password},
		"credentials_type":                 {credentialsType},
		"generate_session_cookies":         {"1"},
		"generate_analytics_claim":         {"1"},
		"method":                           {"auth.login"},
		"try_num":                          {tryNum},
		"cpl":                              {"true"},
		"fb4a_shared_phone_cpl_experiment": {"fb4a_shared_phone_nonce_cpl_at_risk_v3"},
		"fb4a_shared_phone_cpl_group":      {"enable_v3_at_risk"},
		"enroll_misauth":                   {"false"},
		"error_detail_type":                {"button_with_disabled"},
		"source":                           {"login"},
		"currently_logged_in_userid":       {"0"},
		"locale":                           {"es_ES"},
		"client_country_code":              {"ES"},
		"fb_api_req_friendly_name":         {"authenticate"},
		"fb_api_caller_class":              {"Fb4aAuthHandler"},
		"api_key":                          {DefaultFB4AAPIKey},
		"access_token":                     {DefaultFB4AAppAccessToken},
		"jazoest":                          {jazoest},
	}
	if credentialsType == "two_factor" {
		form.Set("sim_serials", "[]")
	}
	return form
}

// FB4ALogin autentica una cuenta de Facebook emulando la app oficial de Android (FB4A).
// Por seguridad y cumplimiento, este método permanece deshabilitado por defecto.
func FB4ALogin(ctx context.Context, client *http.Client, username, password, totpKey string) (*types.LoginSuccessPayload, error) {
	if !FB4ALoginEnabled() {
		return nil, ErrFB4ALoginDisabled
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	deviceID := randomUUID()
	adid := randomUUID()
	machineID := randomAlphaNum(24)

	reqURL := fmt.Sprintf("%s?access_token=%s", FBAuthURL, DefaultFB4AAppAccessToken)
	headers := fb4aHeaders()

	form := buildBaseForm(username, password, "password", "1", deviceID, adid, machineID)

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("error al crear petición de autenticación: %w", err)
	}
	req.Header = headers

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fallo de red en autenticación FB4A: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := readResponseBody(resp)
	if err != nil {
		return nil, fmt.Errorf("error al leer respuesta de login: %w", err)
	}

	var resMap map[string]any
	if err := json.Unmarshal(bodyBytes, &resMap); err != nil {
		return nil, fmt.Errorf("respuesta de login inválida: %w", err)
	}

	// Comprobar éxito directo
	if token, ok := resMap["access_token"].(string); ok && token != "" {
		return buildSuccessPayload(resMap)
	}

	// Comprobar si requiere 2FA
	errMap, _ := resMap["error"].(map[string]any)
	if errMap == nil {
		return nil, &types.FacebookAPIError{
			Code:        -1,
			Title:       "Login failed",
			Description: string(bodyBytes),
		}
	}

	errorCode, _ := errMap["code"].(float64)
	subcode, _ := errMap["error_subcode"].(float64)
	userTitle, _ := errMap["error_user_title"].(string)
	userMsg, _ := errMap["error_user_msg"].(string)
	fbtraceID, _ := errMap["fbtrace_id"].(string)

	if twoFactorSubcodes[int(subcode)] {
		if strings.TrimSpace(totpKey) == "" {
			return nil, &types.FacebookAPIError{
				Code:        int(errorCode),
				Subcode:     int(subcode),
				Title:       "2FA Required",
				Description: "La cuenta requiere autenticación en dos pasos pero no se proporcionó clave 2FA.",
				FBTraceID:   fbtraceID,
			}
		}

		var userID string
		var firstFactor string
		var flowRenderType string

		var challengeURL string
		if errDataRaw, ok := errMap["error_data"]; ok {
			var errDataMap map[string]any
			switch v := errDataRaw.(type) {
			case string:
				_ = json.Unmarshal([]byte(v), &errDataMap)
			case map[string]any:
				errDataMap = v
			}
			if errDataMap != nil {
				if u, ok := errDataMap["url"].(string); ok {
					challengeURL = u
				}
				if uid, ok := errDataMap["uid"].(string); ok {
					userID = uid
				} else if uidNum, ok := errDataMap["uid"].(float64); ok {
					userID = fmt.Sprintf("%.0f", uidNum)
				}
				if uid, ok := errDataMap["userid"].(string); ok && userID == "" {
					userID = uid
				} else if uidNum, ok := errDataMap["userid"].(float64); ok && userID == "" {
					userID = fmt.Sprintf("%.0f", uidNum)
				}
				if uid, ok := errDataMap["user_id"].(string); ok && userID == "" {
					userID = uid
				} else if uidNum, ok := errDataMap["user_id"].(float64); ok && userID == "" {
					userID = fmt.Sprintf("%.0f", uidNum)
				}

				if ff, ok := errDataMap["login_first_factor"].(string); ok {
					firstFactor = ff
				} else if ff, ok := errDataMap["first_factor"].(string); ok {
					firstFactor = ff
				} else if ff, ok := errDataMap["first_factor_id"].(string); ok {
					firstFactor = ff
				}
			}
		}

		totpCode, err := GenerateTOTP(totpKey, time.Now())
		if err != nil {
			return nil, fmt.Errorf("error al generar código 2FA: %w", err)
		}

		// Si Facebook exige resolución vía webview / Bloks checkpoint (sin metadata nativa directa)
		if challengeURL != "" && (firstFactor == "" || userID == "") {
			return nil, &types.TwoFactorChallengeError{
				Code:         int(errorCode),
				Subcode:      int(subcode),
				Title:        userTitle,
				Description:  userMsg,
				ChallengeURL: challengeURL,
				TOTPCode:     totpCode,
				FBTraceID:    fbtraceID,
			}
		}

		// 2. Extraer de two_factor_first_step_metadata si está disponible
		if metaRaw, ok := errMap["two_factor_first_step_metadata"]; ok {
			var metaObj map[string]any
			switch v := metaRaw.(type) {
			case string:
				_ = json.Unmarshal([]byte(v), &metaObj)
			case map[string]any:
				metaObj = v
			}
			if metaObj != nil {
				if firstFactor == "" {
					if ff, ok := metaObj["first_factor"].(string); ok {
						firstFactor = ff
					}
				}
				if flowRenderType == "" {
					if frt, ok := metaObj["flow_render_type"].(string); ok {
						flowRenderType = frt
					}
				}
			}
		}

		secondForm := buildBaseForm(username, totpCode, "two_factor", "2", deviceID, adid, machineID)
		secondForm.Set("twofactor_code", totpCode)
		secondForm.Set("two_factor_code", totpCode)
		if userID != "" {
			secondForm.Set("userid", userID)
		}
		if firstFactor != "" {
			secondForm.Set("first_factor", firstFactor)
		}
		if flowRenderType != "" {
			secondForm.Set("flow_render_type", flowRenderType)
		}

		req2, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(secondForm.Encode()))
		if err != nil {
			return nil, err
		}
		req2.Header = headers

		resp2, err := client.Do(req2)
		if err != nil {
			return nil, fmt.Errorf("fallo de red en paso 2FA: %w", err)
		}
		defer resp2.Body.Close()

		body2Bytes, _ := readResponseBody(resp2)
		var res2Map map[string]any
		_ = json.Unmarshal(body2Bytes, &res2Map)

		if token, ok := res2Map["access_token"].(string); ok && token != "" {
			return buildSuccessPayload(res2Map)
		}

		err2Map, _ := res2Map["error"].(map[string]any)
		msg := "Código 2FA rechazado o expirado"
		if err2Map != nil {
			if m, ok := err2Map["error_user_msg"].(string); ok && m != "" {
				msg = m
			}
		}
		if challengeURL != "" {
			return nil, &types.TwoFactorChallengeError{
				Code:         int(errorCode),
				Subcode:      int(subcode),
				Title:        "2FA Challenge Required",
				Description:  msg,
				ChallengeURL: challengeURL,
				TOTPCode:     totpCode,
				FBTraceID:    fbtraceID,
			}
		}
		return nil, &types.FacebookAPIError{
			Code:        int(errorCode),
			Subcode:     int(subcode),
			Title:       "2FA Verification Failed",
			Description: msg,
			FBTraceID:   fbtraceID,
		}
	}

	return nil, &types.FacebookAPIError{
		Code:        int(errorCode),
		Subcode:     int(subcode),
		Title:       userTitle,
		Description: userMsg,
		FBTraceID:   fbtraceID,
	}
}

func buildSuccessPayload(resMap map[string]any) (*types.LoginSuccessPayload, error) {
	token, _ := resMap["access_token"].(string)
	cookiesList, _ := resMap["session_cookies"].([]any)

	var sb strings.Builder
	var keyValueList []map[string]any

	for _, item := range cookiesList {
		cMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := cMap["name"].(string)
		value, _ := cMap["value"].(string)
		if name != "" && value != "" {
			sb.WriteString(fmt.Sprintf("%s=%s; ", name, value))
			keyValueList = append(keyValueList, cMap)
		}
	}

	return &types.LoginSuccessPayload{
		SetCookies:          sb.String(),
		AccessTokenFB:       token,
		CookiesKeyValueList: keyValueList,
	}, nil
}
