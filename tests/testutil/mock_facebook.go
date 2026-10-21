package testutil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// MockFacebookServer proporciona un servidor HTTP local que simula los endpoints clave de Facebook.
type MockFacebookServer struct {
	Server *httptest.Server
	URL    string
}

// NewMockFacebookServer crea y arranca un MockFacebookServer con respuestas estándar predecibles.
func NewMockFacebookServer() *MockFacebookServer {
	mux := http.NewServeMux()

	// Mock para home / extracción de sesión
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
<script>DTSGInitialData",[],{"token":"mock_dtsg_token_12345"}</script>
<script>{"async_get_token":"mock_dtsg_ag_token"}</script>
</head>
<body>
<input type="hidden" name="jazoest" value="29876"/>
<script>{"sessionId":"mock_session_123","hash":"mock_hash_456"}</script>
<script>{"actorID":"1000888999","client_revision":101827412,}</script>
<script>
{"require":[["ScheduledServerJS","handle",null,[{"mailbox":{"threads":[]}}]]]}
</script>
</body>
</html>
`)
	})

	// Mock para envío de mensajes
	mux.HandleFunc("/messaging/send/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		fmt.Fprintf(w, `for (;;);{"payload":{"actions":[{"message_id":"mid.$mock_sent_msg_123"}]}}`)
	})

	// Mock para GraphQL
	mux.HandleFunc("/api/graphql/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = r.ParseForm()
		docID := r.Form.Get("doc_id")
		fbName := r.Form.Get("fb_api_req_friendly_name")

		if strings.Contains(fbName, "React") || docID == "2972740269476564" {
			fmt.Fprintf(w, `for (;;);{"data":{"message_reaction":{"status":"SUCCESS"}}}`)
			return
		}

		if strings.Contains(fbName, "Theme") {
			fmt.Fprintf(w, `for (;;);{"data":{"messenger_thread_themes":{"theme_edges":[{"node":{"theme_id":"theme_mock","theme_name":"Test Theme","accessibility_label":"Label","theme_key":"key"}}]}}}`)
			return
		}

		fmt.Fprintf(w, `for (;;);{"data":{"viewer":{"actor":{"id":"1000888999"}}}}`)
	})

	// Mock para upload Mercury
	mux.HandleFunc("/ajax/mercury/upload.php", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		fmt.Fprintf(w, `for (;;);{"payload":{"metadata":[{"image_id":"mock_img_555","image_type":"FILE_ATTACHMENT"}]}}`)
	})

	ts := httptest.NewServer(mux)
	return &MockFacebookServer{
		Server: ts,
		URL:    ts.URL,
	}
}

// Close apaga el servidor mock.
func (m *MockFacebookServer) Close() {
	if m.Server != nil {
		m.Server.Close()
	}
}
