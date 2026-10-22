package session

import (
	"strings"
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
	if data.Jazoest != "28912" {
		t.Errorf("jazoest incorrecto: %s", data.Jazoest)
	}
}

func TestParseHomeResponse_MissingTokens(t *testing.T) {
	// HTML incompleto sin DTSG ni sessionID y sin cookies válidas
	incompleteHTML := `<html><body>Login to Facebook</body></html>`
	_, err := ParseHomeResponse(incompleteHTML, "")
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
	// Cookie sin c_user numérico
	_, err := ParseHomeResponse(invalidIDHTML, "datr=abc;")
	if err == nil {
		t.Fatal("ParseHomeResponse debió fallar por FacebookID no numérico")
	}
}

func TestParseSessionData_CometModulesAndWhitespace(t *testing.T) {
	// HTML con espaciado real de JavaScript Comet de Facebook
	cometHTML := `
		<!DOCTYPE html>
		<html>
		<head>
			<script>
				requireLazy(["ScheduledServerJS"], function(s) {
					s.handle(["DTSGInitialData", [], { "token": "AQCometDtsg9876" }, 258]);
					s.handle(["CurrentUserInitialData", [], { "USER_ID": "100055544433322", "ACCOUNT_ID": "100055544433322" }, 259]);
				});
			</script>
		</head>
		<body>
			<script>
				{"client_revision": 101999888, "sessionId": "sess_comet_unique"}
			</script>
		</body>
		</html>
	`
	data, err := ParseSessionData(cometHTML, "xs=test_xs_cookie;")
	if err != nil {
		t.Fatalf("ParseSessionData falló con HTML de Comet: %v", err)
	}

	if data.FbDtsg != "AQCometDtsg9876" {
		t.Errorf("FbDtsg esperado AQCometDtsg9876, obtenido %s", data.FbDtsg)
	}
	if data.FacebookID != "100055544433322" {
		t.Errorf("FacebookID esperado 100055544433322, obtenido %s", data.FacebookID)
	}
	if data.ClientRevision != "101999888" {
		t.Errorf("ClientRevision esperado 101999888, obtenido %s", data.ClientRevision)
	}
	// Comprobar que jazoest se computó automáticamente al no estar en el HTML
	expectedJazoest := ComputeJazoest("AQCometDtsg9876")
	if data.Jazoest != expectedJazoest {
		t.Errorf("Jazoest esperado %s, obtenido %s", expectedJazoest, data.Jazoest)
	}
}

func TestComputeJazoest(t *testing.T) {
	// Prueba del algoritmo: '2' + sum(ASCII)
	// Para "ABC": 'A'(65) + 'B'(66) + 'C'(67) = 198 => "2198"
	result := ComputeJazoest("ABC")
	if result != "2198" {
		t.Errorf("esperado 2198, obtenido %s", result)
	}
}

func TestParseSessionData_LoginOrCheckpointDetection(t *testing.T) {
	loginHTML := `
		<html>
		<body>
			<form id="login_form" action="/login/device-based/regular/login/" method="post">
				<input type="text" name="email" />
			</form>
		</body>
		</html>
	`
	_, err := ParseSessionData(loginHTML, "c_user=123;")
	if err == nil {
		t.Fatal("debió fallar ante HTML de formulario de login")
	}
	if !strings.Contains(err.Error(), "sesión expirada") && !strings.Contains(err.Error(), "facebook solicitó inicio de sesión") {
		t.Errorf("error inesperado: %v", err)
	}
}
