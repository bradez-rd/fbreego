package auth

import (
	"testing"
	"time"
)

func TestValidate2FAKey(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantClean  string
		wantDirect bool
		wantErr    bool
	}{
		{
			name:       "direct 6-digit otp",
			input:      "123456",
			wantClean:  "123456",
			wantDirect: true,
			wantErr:    false,
		},
		{
			name:       "direct 8-digit otp",
			input:      "12345678",
			wantClean:  "12345678",
			wantDirect: true,
			wantErr:    false,
		},
		{
			name:       "valid base32 with spaces",
			input:      "JBSW Y3DP EHPK 3PXP",
			wantClean:  "JBSWY3DPEHPK3PXP",
			wantDirect: false,
			wantErr:    false,
		},
		{
			name:       "valid base32 lowercase with padding",
			input:      "jbswy3dpehpk3pxp====",
			wantClean:  "JBSWY3DPEHPK3PXP",
			wantDirect: false,
			wantErr:    false,
		},
		{
			name:       "empty key",
			input:      "   ",
			wantClean:  "",
			wantDirect: false,
			wantErr:    true,
		},
		{
			name:       "invalid base32 characters",
			input:      "189!invalid",
			wantClean:  "",
			wantDirect: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClean, gotDirect, err := Validate2FAKey(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate2FAKey() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotClean != tt.wantClean {
				t.Errorf("Validate2FAKey() gotClean = %v, want %v", gotClean, tt.wantClean)
			}
			if gotDirect != tt.wantDirect {
				t.Errorf("Validate2FAKey() gotDirect = %v, want %v", gotDirect, tt.wantDirect)
			}
		})
	}
}

func TestGenerateTOTP_RFC6238(t *testing.T) {
	// Secreto canónico RFC 6238 "12345678901234567890" en base32 es "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

	// Timestamp conocido: 59s -> counter = 1
	t1 := time.Unix(59, 0)
	code1, err := GenerateTOTP(secret, t1)
	if err != nil {
		t.Fatalf("GenerateTOTP error: %v", err)
	}
	if len(code1) != 6 {
		t.Fatalf("código TOTP debe tener longitud 6, recibido: %s", code1)
	}

	// Verificar que direct code devuelva el mismo valor
	directCode, err := GenerateTOTP("654321", time.Now())
	if err != nil || directCode != "654321" {
		t.Errorf("GenerateTOTP direct code falló: %s, err: %v", directCode, err)
	}
}
