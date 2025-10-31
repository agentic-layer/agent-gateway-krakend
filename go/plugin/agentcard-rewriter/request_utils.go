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
		return "", fmt.Errorf("Missing host header in request")
	}

	// Default to http, but check X-Forwarded-Proto header
	scheme := "http"
	if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	return fmt.Sprintf("%s://%s", scheme, host), nil
}
