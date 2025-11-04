package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/agentic-layer/agent-gateway-krakend/lib/logging"
)

const (
	pluginName = "agentcard-rw"
)

type registerer string

// HandlerRegisterer is the symbol KrakenD looks up to register plugins
var HandlerRegisterer = registerer(pluginName)
var logger = logging.New(pluginName)

func main() {}

func init() {
	logger.Info("loaded")
}

// RegisterHandlers registers the plugin with KrakenD
func (r registerer) RegisterHandlers(f func(
	name string,
	handler func(context.Context, map[string]interface{}, http.Handler) (http.Handler, error),
)) {
	f(string(r), r.registerHandlers)
	logger.Info("registered")
}

// responseWriter wraps http.ResponseWriter to capture response body
type responseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
		statusCode:     http.StatusOK,
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.body.Write(b)
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
}

func (r registerer) registerHandlers(_ context.Context, _ map[string]interface{}, handler http.Handler) (http.Handler, error) {
	logger.Info("plugin initialized successfully")
	return http.HandlerFunc(r.handleRequest(handler)), nil
}

func (r registerer) handleRequest(handler http.Handler) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		reqLogger := logging.NewWithPluginName(pluginName)

		// Check if this is a GET request to an agent card endpoint
		if req.Method == http.MethodGet && isAgentCardEndpoint(req.URL.Path) {
			reqLogger.Debug("intercepted agent card request: %s", req.URL.Path)

			// Get gateway URL
			gatewayURL, err := getGatewayURL(req)
			if err != nil {
				reqLogger.Error("cannot determine gateway URL: %s", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Extract full agent path from request (everything before /.well-known)
			agentPath := extractAgentPath(req.URL.Path)
			if agentPath == "" {
				reqLogger.Warn("cannot extract agent path from: %s - passing through", req.URL.Path)
				handler.ServeHTTP(w, req)
				return
			}

			reqLogger.Debug("rewriting URLs for agent path: %s, gateway: %s", agentPath, gatewayURL)

			// Wrap response writer to capture backend response
			rw := newResponseWriter(w)

			// Forward request to backend
			handler.ServeHTTP(rw, req)

			// Only transform successful responses
			if rw.statusCode != http.StatusOK {
				reqLogger.Info("backend returned non-OK status: %d - returning error", rw.statusCode)
				http.Error(w, "Backend service returned an error", rw.statusCode)
				return
			}

			// Validate content type
			contentType := rw.Header().Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				reqLogger.Warn("unexpected content-type: %s - returning error", contentType)
				http.Error(w, "Expected application/json content type", http.StatusUnsupportedMediaType)
				return
			}

			// Parse agent card into map to preserve unknown fields
			var agentCardMap map[string]interface{}
			if err := json.Unmarshal(rw.body.Bytes(), &agentCardMap); err != nil {
				reqLogger.Error("failed to parse agent card: %s - returning error", err)
				http.Error(w, "Failed to parse agent card JSON", http.StatusInternalServerError)
				return
			}

			// Rewrite agent card URLs (preserves unknown fields)
			agentCardMap = rewriteAgentCardMap(agentCardMap, gatewayURL, agentPath)

			// Marshal rewritten agent card
			rewrittenBody, err := json.Marshal(agentCardMap)
			if err != nil {
				reqLogger.Error("failed to marshal rewritten agent card: %s", err)
				http.Error(w, "failed to create rewritten agent card", http.StatusInternalServerError)
				return
			}

			reqLogger.Debug("transformed agent card URLs to external gateway format")

			// Remove Content-Length to allow for recalculation
			w.Header().Del("Content-Length")
			w.WriteHeader(http.StatusOK)

			if _, err := w.Write(rewrittenBody); err != nil {
				reqLogger.Error("failed to write response: %s", err)
			}
			return
		}

		// Not an agent card endpoint, pass through
		handler.ServeHTTP(w, req)
	}
}

// getGatewayURL extracts the gateway URL from request headers
// Returns the full URL scheme + host, or an error if Host header is missing
func getGatewayURL(req *http.Request) (string, error) {
	host := req.Host

	// Default to http, but check X-Forwarded-Proto header
	var scheme string
	if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s", scheme, host), nil
}
