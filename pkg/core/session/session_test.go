package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bradez-rd/fbreego/pkg/core/contracts"
	"github.com/bradez-rd/fbreego/pkg/core/types"
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

func TestParseSessionData_MQTTMetadata(t *testing.T) {
	html := `
		<!DOCTYPE html>
		<html>
		<head>
			<script>["DTSGInitialData",[],{"token":"NAcDTSG123"}]</script>
			<script>["CurrentUserInitialData",[],{"USER_ID":"1000111222333"}]</script>
			<script>["MqttWebDeviceID",[],{"clientID":"device-client-uuid-999"}]</script>
			<script>["MqttWebConfig",[],{"appID":219994525426954}]</script>
			<script>{"endpoint":"https:\/\/edge-chat.facebook.com\/chat?region=prn\u0026sid=123"}</script>
		</head>
		<body>
			<input type="hidden" name="jazoest" value="21234" />
			<script>{"sessionId":"session_val","client_revision":101827412}</script>
		</body>
		</html>
	`
	data, err := ParseSessionData(html, "c_user=1000111222333;")
	if err != nil {
		t.Fatalf("ParseSessionData falló: %v", err)
	}

	if data.Region != "prn" {
		t.Errorf("Region esperada 'prn', obtenida '%s'", data.Region)
	}
	if data.MQTTClientID != "device-client-uuid-999" {
		t.Errorf("MQTTClientID esperado 'device-client-uuid-999', obtenido '%s'", data.MQTTClientID)
	}
	if data.MQTTAppID != "219994525426954" {
		t.Errorf("MQTTAppID esperado '219994525426954', obtenido '%s'", data.MQTTAppID)
	}
	if !strings.Contains(data.MQTTEndpoint, "region=prn") {
		t.Errorf("MQTTEndpoint no contiene region=prn: %s", data.MQTTEndpoint)
	}
}

func TestRefreshFbDtsg_Success(t *testing.T) {
	newHTML := `
		<!DOCTYPE html>
		<html>
		<head>
			<script>["DTSGInitialData",[],{"token":"AQNewRefreshedDTSG"}]</script>
			<script>["CurrentUserInitialData",[],{"USER_ID":"1000999888777"}]</script>
			<script>["MqttWebDeviceID",[],{"clientID":"new-device-id"}]</script>
			<script>{"endpoint":"https:\/\/edge-chat.facebook.com\/chat?region=lla\u0026sid=999"}</script>
		</head>
		<body>
			<input type="hidden" name="jazoest" value="29999" />
			<script>{"sessionId":"new_session","client_revision":101827499}</script>
		</body>
		</html>
	`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(newHTML))
	}))
	defer srv.Close()

	// Guardar endpoint previo y restaurarlo
	prevBase := contracts.DefaultRegistry().GetEndpoint("base")
	contracts.DefaultRegistry().SetEndpoint("base", srv.URL)
	defer contracts.DefaultRegistry().SetEndpoint("base", prevBase)

	sessionData := &types.DataFB{
		FacebookID:     "1000999888777",
		FbDtsg:         "AQOldDTSG",
		Jazoest:        "21111",
		CookieFacebook: "c_user=1000999888777; xs=sample_xs;",
	}

	err := RefreshFbDtsg(context.Background(), srv.Client(), sessionData)
	if err != nil {
		t.Fatalf("RefreshFbDtsg falló: %v", err)
	}

	if sessionData.FbDtsg != "AQNewRefreshedDTSG" {
		t.Errorf("FbDtsg esperado 'AQNewRefreshedDTSG', obtenido '%s'", sessionData.FbDtsg)
	}
	if sessionData.Region != "lla" {
		t.Errorf("Region esperada 'lla', obtenida '%s'", sessionData.Region)
	}
	if sessionData.MQTTClientID != "new-device-id" {
		t.Errorf("MQTTClientID esperado 'new-device-id', obtenido '%s'", sessionData.MQTTClientID)
	}
}
