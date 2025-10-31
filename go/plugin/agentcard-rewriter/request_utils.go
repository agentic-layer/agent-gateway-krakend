package main

import (
	"fmt"
	"net/http"
)

// getGatewayURL extracts the gateway URL from request headers
// Returns the full URL scheme + host, or an error if Host header is missing
func getGatewayURL(req *http.Request) (string, error) {
	host := req.Host

	if host == "" {
		return "", fmt.Errorf("Host header is required for agent card URL rewriting")
	}

	// Default to https, but check X-Forwarded-Proto header
	scheme := "https"
	if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	return fmt.Sprintf("%s://%s", scheme, host), nil
}
