package dispatcher

import (
	"net"
	"net/url"
	"strings"
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
