package utils

import (
	"strconv"
	"testing"
)

func TestParseFacebookJSON(t *testing.T) {
	// Caso con prefijo anti-hijacking de Facebook
	payload := []byte(`for (;;);{"code": "SUCCESS", "results": {"count": 42}}`)
	parsed, err := ParseFacebookJSON(payload, true)
	if err != nil {
		t.Fatalf("ParseFacebookJSON falló con prefijo for (;;);: %v", err)
	}
	if parsed["code"] != "SUCCESS" {
		t.Errorf("código inesperado: %v", parsed["code"])
	}

	// Caso sin prefijo
	normalPayload := []byte(`{"status": "ok", "actor_id": "100012345"}`)
	parsedNormal, err := ParseFacebookJSON(normalPayload, true)
	if err != nil {
		t.Fatalf("ParseFacebookJSON falló sin prefijo: %v", err)
	}
	if parsedNormal["actor_id"] != "100012345" {
		t.Errorf("actor_id inesperado: %v", parsedNormal["actor_id"])
	}
}

func TestParseCookieString(t *testing.T) {
	cookieHeader := "c_user=100012345; xs=abc%3Adef; fr=0xyz; presence=ED; "
	cookies := ParseCookieString(cookieHeader)

	if cookies["c_user"] != "100012345" {
		t.Errorf("c_user incorrecto: %s", cookies["c_user"])
	}
	if cookies["xs"] != "abc%3Adef" {
		t.Errorf("xs incorrecto: %s", cookies["xs"])
	}
	if cookies["fr"] != "0xyz" {
		t.Errorf("fr incorrecto: %s", cookies["fr"])
	}
	if cookies["presence"] != "ED" {
		t.Errorf("presence incorrecto: %s", cookies["presence"])
	}
}

func TestClearHTML(t *testing.T) {
	input := `<div class="content"><p>Hello <b>World</b>!</p></div>`
	expected := "Hello World!"
	got := ClearHTML(input)
	if got != expected {
		t.Errorf("ClearHTML got %q, want %q", got, expected)
	}
}

func TestGenThreadingID(t *testing.T) {
	id := GenThreadingID()
	if id == "" {
		t.Fatal("GenThreadingID generó cadena vacía")
	}
	val, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		t.Fatalf("GenThreadingID no es un entero uint64 válido: %v", err)
	}
	if val == 0 {
		t.Errorf("GenThreadingID generó 0")
	}
}

func TestNextRequestBase36(t *testing.T) {
	s1 := NextRequestBase36()
	s2 := NextRequestBase36()
	if s1 == s2 {
		t.Errorf("NextRequestBase36 generó valores idénticos consecutivos: %s", s1)
	}
}
