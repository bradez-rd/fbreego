package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var directOTPRegex = regexp.MustCompile(`^\d{6,8}$`)

// Validate2FAKey limpia y valida una clave secreta base32 o un código OTP directo.
func Validate2FAKey(key string) (cleaned string, isDirectCode bool, err error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "", false, fmt.Errorf("la clave 2FA no puede estar vacía")
	}

	if directOTPRegex.MatchString(trimmed) {
		return trimmed, true, nil
	}

	cleaned = strings.ToUpper(strings.ReplaceAll(trimmed, " ", ""))
	// Quitar padding '=' si existe para normalizar
	cleaned = strings.TrimRight(cleaned, "=")

	// Validar que sea base32 decodificable
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	if _, err := encoder.DecodeString(cleaned); err != nil {
		return "", false, fmt.Errorf("formato de clave secreta 2FA inválido (base32 esperado): %w", err)
	}

	return cleaned, false, nil
}

// GenerateTOTP genera un código TOTP de 6 dígitos para el timestamp actual (ventana de 30s) según RFC 6238.
func GenerateTOTP(secretKey string, t time.Time) (string, error) {
	cleaned, isDirect, err := Validate2FAKey(secretKey)
	if err != nil {
		return "", err
	}
	if isDirect {
		return cleaned, nil
	}

	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	keyBytes, err := encoder.DecodeString(cleaned)
	if err != nil {
		return "", fmt.Errorf("error al decodificar clave base32: %w", err)
	}

	counter := uint64(t.Unix() / 30)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, keyBytes)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	binaryCode := (int64(sum[offset]&0x7f) << 24) |
		(int64(sum[offset+1]&0xff) << 16) |
		(int64(sum[offset+2]&0xff) << 8) |
		(int64(sum[offset+3] & 0xff))

	otp := binaryCode % 1000000
	return fmt.Sprintf("%06d", otp), nil
}
