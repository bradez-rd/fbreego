package utils

import (
	"crypto/rand"
	"math/big"
	"net/http"
)

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36",
}

const secChUA = `"Chromium";v="140", "Not=A?Brand";v="24", "Google Chrome";v="140"`

// RandomUserAgent devuelve un User-Agent de navegador moderno seleccionado pseudoaleatoriamente.
func RandomUserAgent() string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(userAgents))))
	if err != nil {
		return userAgents[0]
	}
	return userAgents[n.Int64()]
}

// DefaultHeaders genera las cabeceras HTTP necesarias para emular peticiones legítimas del navegador a Facebook.
func DefaultHeaders(host string) http.Header {
	if host == "" {
		host = "www.facebook.com"
	}
	h := make(http.Header)
	h.Set("Host", host)
	h.Set("Connection", "keep-alive")
	h.Set("User-Agent", RandomUserAgent())
	h.Set("Accept", "*/*")
	h.Set("Origin", "https://"+host)
	h.Set("Sec-Fetch-Site", "same-origin")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("Sec-Fetch-Dest", "empty")
	h.Set("Sec-Fetch-User", "?1")
	h.Set("x-asbd-id", "129477")
	h.Set("Referer", "https://"+host+"/")
	h.Set("sec-ch-ua", secChUA)
	h.Set("sec-ch-ua-mobile", "?0")
	h.Set("sec-ch-ua-platform", `"Windows"`)
	h.Set("Accept-Language", "en-US,en;q=0.9,vi;q=0.8")
	return h
}
