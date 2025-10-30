package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// isAgentCardEndpoint checks if the path matches the agent card endpoint pattern
func isAgentCardEndpoint(path string) bool {
	return strings.HasSuffix(path, "/.well-known/agent-card.json")
}

// extractAgentPath extracts the full agent path from the request path (everything before /.well-known)
// Examples:
//
//	"/weather-agent/.well-known/agent-card.json" -> "/weather-agent"
//	"/agents/weather-agent/.well-known/agent-card.json" -> "/agents/weather-agent"
//	"/api/v1/agents/weather-agent/.well-known/agent-card.json" -> "/api/v1/agents/weather-agent"
func extractAgentPath(path string) string {
	// Find the position of /.well-known
	idx := strings.Index(path, "/.well-known")
	if idx > 0 {
		return path[:idx]
	}

	// If /.well-known is at the start or not found, return empty
	return ""
}

// getGatewayURL extracts the gateway URL from request headers with config fallback
// Returns the full URL scheme + host, or an error if it cannot be determined
func getGatewayURL(req *http.Request, cfg config) (string, error) {
	host := req.Host

	// Try header detection first - check if host is external (not internal)
	// Extract hostname without port for internal check
	if host != "" {
		// Try to parse to extract hostname (handles both hostname and hostname:port)
		hostname := host
		if h, _, err := net.SplitHostPort(host); err == nil {
			hostname = h
		}

		// If hostname is not internal, use it as gateway URL
		if !isInternalHostname(hostname) {
			// Default to https, but check X-Forwarded-Proto header
			scheme := "https"
			if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
				scheme = proto
			}
			return fmt.Sprintf("%s://%s", scheme, host), nil
		}
	}

	// Fallback to configured gateway URL
	if cfg.GatewayURL != "" {
		return cfg.GatewayURL, nil
	}

	// Both methods failed
	if host == "" {
		return "", fmt.Errorf("Host header not present and no gateway_url configured")
	}
	return "", fmt.Errorf("internal request source and no gateway_url configured")
}

// isPrivateIP checks if an IP address is in a private/internal range
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Check for loopback addresses (localhost)
	if ip.IsLoopback() {
		return true
	}

	// Check for link-local addresses
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check for private IPv4 ranges (RFC 1918)
	// 10.0.0.0/8
	// 172.16.0.0/12
	// 192.168.0.0/16
	privateIPv4Ranges := []struct {
		cidr string
	}{
		{"10.0.0.0/8"},
		{"172.16.0.0/12"},
		{"192.168.0.0/16"},
		{"169.254.0.0/16"}, // Link-local (also caught by IsLinkLocalUnicast but being explicit)
	}

	for _, r := range privateIPv4Ranges {
		_, subnet, err := net.ParseCIDR(r.cidr)
		if err != nil {
			continue
		}
		if subnet.Contains(ip) {
			return true
		}
	}

	// Check for IPv6 Unique Local Addresses (ULA) - fc00::/7
	if len(ip) == net.IPv6len && ip[0] >= 0xfc && ip[0] <= 0xfd {
		return true
	}

	return false
}

// isInternalHostname checks if a hostname is internal/local
func isInternalHostname(hostname string) bool {
	if hostname == "" {
		return false
	}

	hostname = strings.ToLower(hostname)

	// Check for localhost
	if hostname == "localhost" {
		return true
	}

	// Check for Kubernetes cluster domains
	// Full form: service.namespace.svc.cluster.local
	if strings.Contains(hostname, ".svc.cluster.local") {
		return true
	}

	// Kubernetes short-form service names (no dots = same namespace)
	// or service.namespace (cross-namespace, no .svc or .cluster.local)
	// Pattern: Simple hostname with no domain suffix
	// Examples: "weather-agent", "weather-agent.default"
	// We need to be careful not to match public domains

	// If it ends with .svc (without cluster.local), it's internal
	if strings.HasSuffix(hostname, ".svc") {
		return true
	}

	// Check for Docker internal hostnames
	if strings.HasSuffix(hostname, ".docker.internal") {
		return true
	}

	// Check for special-use domains
	// .local (mDNS/Bonjour - RFC 6762)
	// .internal (common convention)
	// .localhost (RFC 2606)
	// BUT: Exclude .cluster.local (that's only internal with .svc)
	if !strings.HasSuffix(hostname, ".cluster.local") {
		if strings.HasSuffix(hostname, ".local") ||
			strings.HasSuffix(hostname, ".internal") ||
			strings.HasSuffix(hostname, ".localhost") {
			return true
		}
	}

	// Check for Kubernetes cross-namespace short-form: service.namespace
	// This has exactly one dot, and both parts should be valid K8s names
	// Examples: "weather-agent.default", "api-gateway.production"
	// BUT: Exclude if it looks like a public domain (e.g., company.com, qaware.de)
	dotCount := strings.Count(hostname, ".")
	if dotCount == 1 {
		parts := strings.Split(hostname, ".")
		if len(parts) == 2 {
			// Check if the second part looks like a TLD (2-3 chars typically)
			// Common TLDs: com, org, net, de, uk, io, etc.
			secondPart := parts[1]
			if len(secondPart) >= 2 && len(secondPart) <= 3 {
				// Likely a TLD, not a K8s namespace
				return false
			}

			// Both parts should be valid K8s service/namespace names
			allValid := true
			for _, part := range parts {
				if len(part) == 0 {
					allValid = false
					break
				}
				// Check if it's a valid K8s name (alphanumeric and hyphens)
				for _, ch := range part {
					if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
						allValid = false
						break
					}
				}
				if !allValid {
					break
				}
			}
			if allValid {
				return true
			}
		}
	}

	// Check for simple hostnames (no dots) - likely Kubernetes service in same namespace
	// BUT: be careful not to match malformed URLs
	// Only match if it looks like a valid hostname (alphanumeric, hyphens, no spaces)
	if dotCount == 0 {
		// Simple validation: only lowercase letters, numbers, hyphens
		isValidSimpleName := true
		for _, ch := range hostname {
			if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
				isValidSimpleName = false
				break
			}
		}
		if isValidSimpleName && len(hostname) > 0 {
			return true
		}
	}

	return false
}

// isInternalURL checks if a URL is an internal/local URL that should be rewritten
// This includes:
// - Kubernetes cluster URLs (.svc.cluster.local, short-form service names)
// - Localhost and loopback addresses
// - Private IP ranges (RFC 1918)
// - Docker internal hostnames
// - Special-use domains (.local, .internal, .localhost)
// - Link-local addresses
func isInternalURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}

	// Parse the URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// If we can't parse it, fall back to simple string matching for .svc.cluster.local
		return strings.Contains(urlStr, ".svc.cluster.local")
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return false
	}

	// Try to parse as IP address first
	ip := net.ParseIP(hostname)
	if ip != nil {
		// It's an IP address - check if it's private/internal
		return isPrivateIP(ip)
	}

	// It's a hostname - check if it's internal
	return isInternalHostname(hostname)
}

// copyHeaders copies all headers from source to destination
func copyHeaders(dst, src http.Header) {
	for k, v := range src {
		dst[k] = v
	}
}
