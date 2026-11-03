package utils

import (
	"net/http"
)

const (
	ConsistentUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
	secChUA             = `"Chromium";v="140", "Not=A?Brand";v="24", "Google Chrome";v="140"`
)

// RandomUserAgent devuelve el User-Agent estándar y consistente para la sesión.
func RandomUserAgent() string {
	return ConsistentUserAgent
}

// DefaultHeaders genera las cabeceras HTTP necesarias para emular peticiones legítimas del navegador a Facebook.
func DefaultHeaders(host string) http.Header {
	if host == "" {
		host = "www.facebook.com"
	}
	h := make(http.Header)
	h.Set("Host", host)
	h.Set("Connection", "keep-alive")
	h.Set("User-Agent", ConsistentUserAgent)
	h.Set("Accept", "*/*")
	h.Set("Origin", "https://"+host)
	h.Set("Sec-Fetch-Site", "same-origin")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("Sec-Fetch-Dest", "empty")
	h.Set("Referer", "https://"+host+"/")
	h.Set("sec-ch-ua", secChUA)
	h.Set("sec-ch-ua-mobile", "?0")
	h.Set("sec-ch-ua-platform", `"Windows"`)
	h.Set("Accept-Language", "es-ES,es;q=0.9,en-US;q=0.8,en;q=0.7")
	return h
}
