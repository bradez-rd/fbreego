package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/types"
)

const (
	FBAuthURL                  = "https://b-graph.facebook.com/auth/login"
	DefaultFB4AAPIKey          = "882a8490361da98702bf97a021ddc14d"
	DefaultFB4AAppAccessToken  = "350685531728|62f8ce9f74b12f84c123cc23437a4a32"
	LegacyFB4AUserAgent        = "Dalvik/2.1.0 (Linux; U; Android 7.1.2; SM-G988N Build/NRD90M) [FBAN/FB4A;FBAV/340.0.0.27.113;FBPN/com.facebook.katana;FBLC/en_US;FBBV/324485361;FBCR/Carrier;FBMF/samsung;FBBD/samsung;FBDV/SM-G988N;FBSV/7.1.2;FBCA/x86:armeabi-v7a;FBDM/{density=1.0,width=540,height=960};FB_FW/1;FBRV/0;]"
)

var twoFactorSubcodes = map[int]bool{
	1348162: true,
	1348023: true,
}

// randomUUID genera un UUID v4 pseudoaleatorio para los campos de dispositivo de FB4A.
func randomUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// FB4ALogin autentica una cuenta de Facebook emulando la app oficial de Android (FB4A).
func FB4ALogin(ctx context.Context, client *http.Client, username, password, totpKey string) (*types.LoginSuccessPayload, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	deviceID := randomUUID()
	adid := randomUUID()

	form := url.Values{
		"adid":                     {adid},
		"format":                   {"json"},
		"device_id":                {deviceID},
		"email":                    {username},
		"password":                 {password},
		"credentials_type":         {"password"},
		"generate_session_cookies": {"1"},
		"generate_analytics_claim": {"1"},
		"method":                   {"auth.login"},
		"try_num":                  {"1"},
	}

	headers := http.Header{
		"User-Agent":      {LegacyFB4AUserAgent},
		"Content-Type":    {"application/x-www-form-urlencoded"},
		"Accept-Language": {"en-US,en;q=0.9"},
	}

	reqURL := fmt.Sprintf("%s?access_token=%s", FBAuthURL, DefaultFB4AAppAccessToken)
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

	bodyBytes, err := io.ReadAll(resp.Body)
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

		// Extraer metadata 2FA
		metaStr, _ := errMap["two_factor_first_step_metadata"].(string)
		var metaObj map[string]any
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &metaObj)
		}
		firstFactor, _ := metaObj["first_factor"].(string)
		flowRenderType, _ := metaObj["flow_render_type"].(string)

		totpCode, err := GenerateTOTP(totpKey, time.Now())
		if err != nil {
			return nil, fmt.Errorf("error al generar código 2FA: %w", err)
		}

		secondForm := url.Values{
			"adid":                     {adid},
			"format":                   {"json"},
			"device_id":                {deviceID},
			"email":                    {username},
			"password":                 {password},
			"credentials_type":         {"two_factor"},
			"two_factor_code":          {totpCode},
			"first_factor":             {firstFactor},
			"flow_render_type":         {flowRenderType},
			"generate_session_cookies": {"1"},
			"generate_analytics_claim": {"1"},
			"method":                   {"auth.login"},
			"try_num":                  {"2"},
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

		body2Bytes, _ := io.ReadAll(resp2.Body)
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
