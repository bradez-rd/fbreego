package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

var cookiesAccountB = `datr=4xHcap9NGNgD_8CvAl604o9t; fr=1IsbYI7EzBKa7I0su.AWdvwVPOCwL48AT50WLmMFm___u6QG2AcXOs9pOFYsZ8G0iphdg.Bq3EYb..AAA.0.0.Bq3EYb.AWce1zIdytJ_TvtCGvYpqvywXnQ; xs=32%3ATI9qwv8V9gvUpw%3A2%3A1792807619%3A-1%3A-1%3A%3AAcxOYoOVorDfNEbfzCk4PIvazDaov35cNgOLyEYwVA; c_user=100011614426020; sb=5xHcap2NrHOWP4M-BlvEuPq_`

func main() {
	fmt.Println("==========================================================")
	fmt.Println("🔑 INSPECCIÓN Y EXTRACCIÓN DE LLAVES E2EE (CAT & ICDC)")
	fmt.Println("==========================================================")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 45 * time.Second}

	targetURL := "https://www.facebook.com/messages/e2ee/t/1305701081633035/"
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	headers := utils.DefaultHeaders("www.facebook.com")
	for k, v := range headers {
		req.Header[k] = v
	}
	req.Header.Set("Cookie", cookiesAccountB)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	fmt.Printf("1. Descargando HTML del hilo E2EE (%s)...\n", targetURL)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error requesting thread HTML: %v\n", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading body: %v\n", err)
		return
	}

	html := string(bodyBytes)
	fmt.Printf("   HTTP Status: %d, Tamaño recibido: %d bytes\n", resp.StatusCode, len(html))

	// Buscar bloques de configuración
	reCAT := regexp.MustCompile(`"encrypted_serialized_cat":\s*"([^"]+)"`)
	matchCAT := reCAT.FindStringSubmatch(html)
	if len(matchCAT) > 1 {
		cat := matchCAT[1]
		fmt.Printf("✅ CAT Encontrado en HTML: %s... (longitud: %d)\n", cat[:40], len(cat))
	} else {
		fmt.Println("⚠️ 'encrypted_serialized_cat' no encontrado directamente por regex simple.")
	}

	reInit := regexp.MustCompile(`\["MessengerWebInitData",\[\],({.*?})\s*,\s*\d+\]`)
	matchInit := reInit.FindStringSubmatch(html)
	if len(matchInit) > 1 {
		fmt.Printf("✅ MessengerWebInitData JSON encontrado:\n%s\n", matchInit[1])
	} else {
		fmt.Println("⚠️ MessengerWebInitData no encontrado con regex estándar, buscando 'MessengerWebInitData'...")
		idx := strings.Index(html, "MessengerWebInitData")
		if idx != -1 {
			start := idx - 20
			if start < 0 {
				start = 0
			}
			end := idx + 300
			if end > len(html) {
				end = len(html)
			}
			fmt.Printf("   Snippet de MessengerWebInitData: %q\n", html[start:end])
		} else {
			fmt.Println("❌ 'MessengerWebInitData' no aparece en el HTML de esta página.")
		}
	}

	// Buscar cualquier mención de CAT o Armadillo
	reArmadillo := regexp.MustCompile(`"CryptoAuthToken":\s*({[^}]+})`)
	matchArm := reArmadillo.FindStringSubmatch(html)
	if len(matchArm) > 1 {
		fmt.Printf("✅ CryptoAuthToken: %s\n", matchArm[1])
	}

	fmt.Println("\n==========================================================")
}
