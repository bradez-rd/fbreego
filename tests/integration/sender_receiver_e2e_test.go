package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/bradez-rd/fbreego/pkg/core/types"
)

func TestSenderReceiver_E2EFlow(t *testing.T) {
	ts, _, cleanup := setupTestServer(t)
	defer cleanup()

	client := ts.Client()

	// 1. Crear Sesión Emisora (Alice - Sender)
	var senderSessionID string
	t.Run("Create Sender Session (Alice)", func(t *testing.T) {
		senderPayload := types.SessionCreateInput{
			Label:    "Sender_Alice",
			TenantID: "default",
			Cookies: []types.CookieInput{
				{Name: "c_user", Value: "1000888999"},
				{Name: "xs", Value: "alice_session_token_123"},
			},
		}
		raw, _ := json.Marshal(senderPayload)
		resp, err := client.Post(ts.URL+"/api/v1/sessions", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("failed to create sender session: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200/201, got %d: %s", resp.StatusCode, string(body))
		}

		var res map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&res)
		data, _ := res["results"].(map[string]any)
		if data != nil {
			senderSessionID, _ = data["sessionId"].(string)
		}
		if senderSessionID == "" {
			t.Fatalf("missing sender sessionId: %v", res)
		}
	})

	// 2. Crear Sesión Receptora (Bob - Receiver)
	var receiverSessionID string
	t.Run("Create Receiver Session (Bob)", func(t *testing.T) {
		receiverPayload := types.SessionCreateInput{
			Label:    "Receiver_Bob",
			TenantID: "default",
			Cookies: []types.CookieInput{
				{Name: "c_user", Value: "1000777888"},
				{Name: "xs", Value: "bob_session_token_456"},
			},
		}
		raw, _ := json.Marshal(receiverPayload)
		resp, err := client.Post(ts.URL+"/api/v1/sessions", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("failed to create receiver session: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200/201, got %d: %s", resp.StatusCode, string(body))
		}

		var res map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&res)
		data, _ := res["results"].(map[string]any)
		if data != nil {
			receiverSessionID, _ = data["sessionId"].(string)
		}
		if receiverSessionID == "" {
			t.Fatalf("missing receiver sessionId: %v", res)
		}
	})

	// 3. Validar Estado de Ambas Sesiones
	t.Run("Validate Sessions Status", func(t *testing.T) {
		respS, err := client.Get(ts.URL + "/api/v1/sessions/" + senderSessionID + "/status")
		if err != nil || respS.StatusCode != http.StatusOK {
			t.Fatalf("sender status check failed: %v", err)
		}
		defer respS.Body.Close()

		respR, err := client.Get(ts.URL + "/api/v1/sessions/" + receiverSessionID + "/status")
		if err != nil || respR.StatusCode != http.StatusOK {
			t.Fatalf("receiver status check failed: %v", err)
		}
		defer respR.Body.Close()
	})

	// 4. Enviar Mensaje de Texto (Sender -> Receiver)
	var sentMessageID string
	t.Run("Send Text Message from Sender to Receiver", func(t *testing.T) {
		msgPayload := types.TextMessageInput{
			ThreadID:   "1000777888",
			ThreadType: "user",
			Text:       "Hola Bob! Prueba de mensajeria entre sesiones fbreego",
		}
		raw, _ := json.Marshal(msgPayload)
		resp, err := client.Post(ts.URL+"/api/v1/sessions/"+senderSessionID+"/messages/text", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("send text failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200 on send text, got %d: %s", resp.StatusCode, string(body))
		}

		var res map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&res)
		data, _ := res["results"].(map[string]any)
		if data != nil {
			if payload, ok := data["payload"].(map[string]any); ok {
				if actions, ok := payload["actions"].([]any); ok && len(actions) > 0 {
					if act, ok := actions[0].(map[string]any); ok {
						sentMessageID, _ = act["message_id"].(string)
					}
				}
			}
		}
		if sentMessageID == "" {
			sentMessageID = "mid.$mock_sent_msg_123"
		}
	})

	// 5. Enviar Respuesta Citando el Mensaje
	t.Run("Send Reply Quoting Previous Message", func(t *testing.T) {
		replyPayload := types.TextMessageInput{
			ThreadID:         "1000777888",
			ThreadType:       "user",
			Text:             "Esta es una respuesta citando el mensaje previo",
			ReplyToMessageID: sentMessageID,
		}
		raw, _ := json.Marshal(replyPayload)
		resp, err := client.Post(ts.URL+"/api/v1/sessions/"+senderSessionID+"/messages/text", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("send reply failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 on reply, got %d", resp.StatusCode)
		}
	})

	// 6. Agregar Reacción al Mensaje
	t.Run("Add Reaction to Message", func(t *testing.T) {
		reactPayload := types.ReactionInput{
			MessageID: sentMessageID,
			Emoji:     "👍",
			Action:    "add",
		}
		raw, _ := json.Marshal(reactPayload)
		resp, err := client.Post(ts.URL+"/api/v1/sessions/"+senderSessionID+"/messages/reaction", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("add reaction failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 on reaction, got %d", resp.StatusCode)
		}
	})

	// 7. Revocar Mensaje (Unsend)
	t.Run("Unsend Message for Everyone", func(t *testing.T) {
		unsendPayload := types.UnsendInput{
			MessageID: sentMessageID,
		}
		raw, _ := json.Marshal(unsendPayload)
		resp, err := client.Post(ts.URL+"/api/v1/sessions/"+senderSessionID+"/messages/unsend", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("unsend failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 on unsend, got %d", resp.StatusCode)
		}
	})

	// 8. Ejecutar la Suite Completa de Tests E2E via API Endpoint
	t.Run("Execute POST /api/v1/tests/e2e Suite", func(t *testing.T) {
		testReq := map[string]any{
			"senderSessionId":   senderSessionID,
			"receiverSessionId": receiverSessionID,
			"targetThreadId":    "1000777888",
			"threadType":        "user",
			"includeMedia":      false,
		}
		raw, _ := json.Marshal(testReq)
		resp, err := client.Post(ts.URL+"/api/v1/tests/e2e", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("e2e test suite failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200 on e2e test suite, got %d: %s", resp.StatusCode, string(body))
		}

		var res map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&res)
		results, _ := res["results"].(map[string]any)
		if results == nil {
			t.Fatalf("missing results in e2e suite response: %v", res)
		}

		failed, _ := results["failedSteps"].(float64)
		if failed > 0 {
			t.Errorf("expected 0 failed steps, got %v: %v", failed, results["steps"])
		}

		passed, _ := results["passedSteps"].(float64)
		if passed < 5 {
			t.Errorf("expected at least 5 passed steps, got %v", passed)
		}
	})

	// 9. Verificar Tráfico de Red Saliente Capturado
	t.Run("Verify Outbound Facebook Network Traffic", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/v1/traffic?direction=OUTBOUND&limit=20")
		if err != nil {
			t.Fatalf("get traffic failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 on traffic, got %d", resp.StatusCode)
		}

		var trResp map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&trResp)
		records, _ := trResp["results"].([]any)
		if len(records) == 0 {
			t.Errorf("expected captured outbound traffic records, got 0")
		}
	})
}
