package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/session"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/features/thread"
	"github.com/bradez-rd/fbreego/pkg/messaging/send"
)

var cookiesAccountA = `[
    { "name": "ps_l", "value": "1" },
    { "name": "datr", "value": "mg_calcZz1axpn9vRXq5gM9T" },
    { "name": "fr", "value": "1F5mEjuKx67vjzJk5.AWdm46EvIMklYKck04aaN78Aun5ZYLaWBPYt0ViSZovIKisvbnU.Bq3CBi..AAA.0.0.Bq3CBi.AWeoJjT8QoTJpp6eyFF8rU5L4RQ" },
    { "name": "xs", "value": "39%3A0xEgDMP5Z__Zbw%3A2%3A1792806951%3A-1%3A-1%3A%3AAcxWXKka6hDN6YYSYqGBimZnnUK2jPAZRyinIFU0-Q" },
    { "name": "c_user", "value": "61595947434077" },
    { "name": "presence", "value": "C%7B%22t3%22%3A%5B%5D%2C%22utc3%22%3A1792806963399%2C%22v%22%3A1%7D" },
    { "name": "ps_n", "value": "1" },
    { "name": "sb", "value": "mg_cajqp2qB5HP1UiIkVZKii" },
    { "name": "wd", "value": "1912x956" }
]`

var cookiesAccountB = `[
    { "name": "ps_l", "value": "1" },
    { "name": "datr", "value": "4xHcap9NGNgD_8CvAl604o9t" },
    { "name": "fr", "value": "1IsbYI7EzBKa7I0su.AWdvwVPOCwL48AT50WLmMFm___u6QG2AcXOs9pOFYsZ8G0iphdg.Bq3EYb..AAA.0.0.Bq3EYb.AWce1zIdytJ_TvtCGvYpqvywXnQ" },
    { "name": "xs", "value": "32%3ATI9qwv8V9gvUpw%3A2%3A1792807619%3A-1%3A-1%3A%3AAcxJd9HdHZG0wGuPec6CYyPNesD45ipyOVCBc4EH0Q" },
    { "name": "locale", "value": "en_US" },
    { "name": "c_user", "value": "100011614426020" },
    { "name": "presence", "value": "C%7B%22t3%22%3A%5B%5D%2C%22utc3%22%3A1792823744104%2C%22v%22%3A1%7D" },
    { "name": "ps_n", "value": "1" },
    { "name": "sb", "value": "5xHcap2NrHOWP4M-BlvEuPq_" },
    { "name": "wd", "value": "1920x919" }
]`

func buildCookieHeader(rawJSON string) string {
	var list []types.CookieInput
	_ = json.Unmarshal([]byte(rawJSON), &list)
	header := ""
	for _, c := range list {
		header += fmt.Sprintf("%s=%s; ", c.Name, c.Value)
	}
	return header
}

func main() {
	fmt.Println("==========================================================")
	fmt.Println("💬 TEST DE ENVÍO DE MENSAJES BIDIRECCIONAL ENTRE CUENTAS")
	fmt.Println("==========================================================")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	httpClient := &http.Client{
		Timeout: 45 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			fmt.Printf("   [HTTP REDIRECT %d] %s -> %s (Method: %s)\n", len(via), via[len(via)-1].URL.String(), req.URL.String(), req.Method)
			return nil
		},
	}

	// 1. Inicializar sesión de Cuenta A (61595947434077 - Miguelest)
	fmt.Println("\n[1/4] Extrayendo sesión para Cuenta A (61595947434077)...")
	dataA, errA := session.ExtractSession(ctx, httpClient, buildCookieHeader(cookiesAccountA), nil)
	if errA != nil {
		fmt.Printf("❌ Falló sesión Cuenta A: %v\n", errA)
		os.Exit(1)
	}
	fmt.Printf("✅ Cuenta A activa: ID=%s, DTSG=%s...\n", dataA.FacebookID, dataA.FbDtsg[:15])

	// 2. Inicializar sesión de Cuenta B (100011614426020 - Brailin)
	fmt.Println("\n[2/4] Extrayendo sesión para Cuenta B (100011614426020)...")
	dataB, errB := session.ExtractSession(ctx, httpClient, buildCookieHeader(cookiesAccountB), nil)
	if errB != nil {
		fmt.Printf("❌ Falló sesión Cuenta B: %v\n", errB)
		os.Exit(1)
	}
	fmt.Printf("✅ Cuenta B activa: ID=%s, DTSG=%s...\n", dataB.FacebookID, dataB.FbDtsg[:15])

	const liveThreadID = "1305701081633035"

	// 2.5 Verificar lectura del mensaje que el usuario acaba de enviar desde el navegador
	fmt.Printf("\n[LECTURA] Verificando mensajes existentes en el hilo E2EE (%s)...\n", liveThreadID)
	msgsPre, errFetchPre := thread.FetchThreadMessages(ctx, httpClient, dataB, liveThreadID, 5, "")
	if errFetchPre != nil {
		fmt.Printf("⚠️ No se pudo verificar historial previo: %v\n", errFetchPre)
	} else {
		fmt.Printf("✅ Mensajes confirmados en el hilo (%d mensajes):\n", len(msgsPre))
		for i, m := range msgsPre {
			fmt.Printf("   [%d] De: %s | Texto: %q | MsgID: %s\n", i+1, m.SenderID, m.Text, m.MessageID)
		}
	}

	// 3. Enviar mensaje de Cuenta B -> Cuenta A en el hilo 1305701081633035
	msgTextBtoA := fmt.Sprintf("Hola Miguel! Prueba de mensaje en vivo desde fbreego (B -> A) [%s]", time.Now().Format("15:04:05"))
	fmt.Printf("\n[3/4] Enviando mensaje de Cuenta B (100011614426020) en hilo (%s):\n      Texto: %q\n", liveThreadID, msgTextBtoA)

	inputB := types.TextMessageInput{
		ThreadID:   liveThreadID,
		Text:       msgTextBtoA,
		ThreadType: "lightspeed",
	}
	respB, errSendB := send.SendText(ctx, httpClient, dataB, inputB)
	if errSendB != nil {
		fmt.Printf("❌ Error al enviar mensaje B -> A: %v\n", errSendB)
	} else {
		fmt.Printf("✅ Mensaje enviado exitosamente de B a A!\n")
		bRespBytes, _ := json.Marshal(respB)
		fmt.Printf("   Respuesta Meta: %s\n", string(bRespBytes))
	}

	// Pausa prudente de 3 segundos para emular ritmo humano y proteger las cuentas
	fmt.Println("\n⏳ Esperando 3 segundos antes de responder...")
	time.Sleep(3 * time.Second)

	// 4. Enviar mensaje de respuesta de Cuenta A -> Cuenta B en el hilo 1305701081633035
	msgTextAtoB := fmt.Sprintf("Hola Brailin! Confirmando recepcion desde fbreego (A -> B) [%s]", time.Now().Format("15:04:05"))
	fmt.Printf("\n[4/4] Enviando respuesta de Cuenta A (61595947434077) en hilo (%s):\n      Texto: %q\n", liveThreadID, msgTextAtoB)

	inputA := types.TextMessageInput{
		ThreadID:   liveThreadID,
		Text:       msgTextAtoB,
		ThreadType: "lightspeed",
	}
	respA, errSendA := send.SendText(ctx, httpClient, dataA, inputA)
	if errSendA != nil {
		fmt.Printf("❌ Error al enviar respuesta A -> B: %v\n", errSendA)
	} else {
		fmt.Printf("✅ Mensaje enviado exitosamente de A a B!\n")
		aRespBytes, _ := json.Marshal(respA)
		fmt.Printf("   Respuesta Meta: %s\n", string(aRespBytes))
	}

	// 5. Verificar lectura final del hilo
	fmt.Println("\n[EXTRA] Verificando historial final de mensajes en el hilo...")
	time.Sleep(2 * time.Second)
	msgs, errFetch := thread.FetchThreadMessages(ctx, httpClient, dataB, liveThreadID, 5, "")
	if errFetch != nil {
		fmt.Printf("⚠️ No se pudo verificar historial: %v\n", errFetch)
	} else {
		fmt.Printf("✅ Mensajes confirmados en el hilo (%d mensajes):\n", len(msgs))
		for i, m := range msgs {
			fmt.Printf("   [%d] De: %s | Texto: %q\n", i+1, m.SenderID, m.Text)
		}
	}

	fmt.Println("\n==========================================================")
	fmt.Println("🏁 PRUEBA DE ENVÍO Y RECEPCIÓN COMPLETADA")
	fmt.Println("==========================================================")
}
