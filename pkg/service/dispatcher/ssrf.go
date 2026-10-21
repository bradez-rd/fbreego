package dispatcher

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var privateCIDRs []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",    // IPv4 loopback
		"::1/128",        // IPv6 loopback
		"10.0.0.0/8",     // RFC1918 privado
		"172.16.0.0/12",  // RFC1918 privado
		"192.168.0.0/16", // RFC1918 privado
		"169.254.0.0/16", // IPv4 Link-local y metadatos cloud (169.254.169.254)
		"fe80::/10",      // IPv6 Link-local
		"fc00::/7",       // IPv6 Unique local
		"0.0.0.0/8",      // Red local
		"224.0.0.0/4",    // Multicast
		"240.0.0.0/4",    // Reservado
		"100.64.0.0/10",  // Shared address space / CGNAT
		"192.0.0.0/24",   // IETF protocol assignments
		"192.0.2.0/24",   // Documentation
		"198.18.0.0/15",  // Benchmarking
		"198.51.100.0/24", // Documentation
		"203.0.113.0/24", // Documentation
		"ff00::/8",       // IPv6 multicast
		"2001:db8::/32",  // IPv6 documentation
	}

	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			privateCIDRs = append(privateCIDRs, ipNet)
		}
	}
}

// IsSSRFRisk valida si una dirección URL o nombre de host apunta a una red interna, privada o servicio de metadatos.
func IsSSRFRisk(targetURL string) bool {
	u, err := url.Parse(targetURL)
	if err != nil {
		return true
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return true
	}
	if port := u.Port(); port != "" && ((u.Scheme == "http" && port != "80") || (u.Scheme == "https" && port != "443")) {
		return true
	}

	host := u.Hostname()
	if host == "" {
		host = targetURL
	}
	host = strings.Trim(host, "[]")

	// Si es una IP directa
	if ip := net.ParseIP(host); ip != nil {
		return isPrivateIP(ip)
	}

	// Si es "localhost"
	if strings.EqualFold(host, "localhost") {
		return true
	}

	// Resolución DNS para evitar SSRF por DNS rebinding o nombres resolviendo a IPs privadas
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return true // Fallar de forma segura si no resuelve
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return true
		}
	}

	return false
}

func newSafeHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           safeDialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("demasiadas redirecciones de webhook")
			}
			if IsSSRFRisk(req.URL.String()) {
				return fmt.Errorf("redirección de webhook bloqueada por política anti-SSRF")
			}
			return nil
		},
	}
}

// safeDialContext resolves and validates the exact IP that will be dialed, closing
// the DNS validation/use gap that would otherwise permit rebinding attacks.
func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("dirección de webhook inválida: %w", err)
	}
	resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(resolved) == 0 {
		return nil, fmt.Errorf("no se pudo resolver el host del webhook")
	}
	for _, item := range resolved {
		if isPrivateIP(item.IP) {
			return nil, fmt.Errorf("el host del webhook resolvió a una red privada")
		}
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(resolved[0].IP.String(), port))
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
		return true
	}

	for _, cidr := range privateCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}
