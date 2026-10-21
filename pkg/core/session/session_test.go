package session

import (
	"testing"
)

func TestParseHomeResponse_Valid(t *testing.T) {
	fixtureHTML := `
		<!DOCTYPE html>
		<html>
		<head>
			<script>DTSGInitialData",[],{"token":"NAcTestDTSGToken123456"}</script>
			<script>{"async_get_token":"async_token_xyz"}</script>
		</head>
		<body>
			<input type="hidden" name="jazoest" value="28912" />
			<script>{"sessionId":"session_abc_999","hash":"hash_token_456"}</script>
			<script>{"actorID":"1000987654321","client_revision":101827412,}</script>
		</body>
		</html>
	`
	cookieStr := "c_user=1000987654321; xs=valid_session_cookie; "

	data, err := ParseHomeResponse(fixtureHTML, cookieStr)
	if err != nil {
		t.Fatalf("ParseHomeResponse falló inesperadamente: %v", err)
	}

	if data.FbDtsg != "NAcTestDTSGToken123456" {
		t.Errorf("fb_dtsg incorrecto: %s", data.FbDtsg)
	}
	if data.SessionID != "session_abc_999" {
		t.Errorf("sessionID incorrecto: %s", data.SessionID)
	}
	if data.FacebookID != "1000987654321" {
		t.Errorf("FacebookID incorrecto: %s", data.FacebookID)
	}
	if data.ClientRevision != "101827412" {
		t.Errorf("clientRevision incorrecto: %s", data.ClientRevision)
	}
}

func TestParseHomeResponse_MissingTokens(t *testing.T) {
	// HTML incompleto sin DTSG ni sessionID
	incompleteHTML := `<html><body>Login to Facebook</body></html>`
	_, err := ParseHomeResponse(incompleteHTML, "c_user=10001;")
	if err == nil {
		t.Fatal("ParseHomeResponse debió fallar por tokens faltantes")
	}
}

func TestParseHomeResponse_NonNumericFacebookID(t *testing.T) {
	invalidIDHTML := `
		<script>DTSGInitialData",[],{"token":"tok123"}</script>
		<input name="jazoest" value="21" />
		<script>{"sessionId":"sess1","hash":"h1","actorID":"not_a_number","client_revision":1234,}</script>
	`
	_, err := ParseHomeResponse(invalidIDHTML, "c_user=123;")
	if err == nil {
		t.Fatal("ParseHomeResponse debió fallar por FacebookID no numérico")
	}
}
