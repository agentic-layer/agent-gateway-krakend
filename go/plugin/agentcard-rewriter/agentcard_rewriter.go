package main

import (
	"bytes"
	"context"
	"encoding/json"
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

func (r registerer) registerHandlers(_ context.Context, extra map[string]interface{}, handler http.Handler) (http.Handler, error) {
	logger.Info("plugin initialized successfully")
	return http.HandlerFunc(r.handleRequest(handler)), nil
}

func (r registerer) handleRequest(handler http.Handler) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		reqLogger := logging.NewWithPluginName(pluginName)

		// Check if this is a GET request to an agent card endpoint
		if req.Method == http.MethodGet && isAgentCardEndpoint(req.URL.Path) {
			reqLogger.Debug("intercepted agent card request: %s", req.URL.Path)

			// Get gateway URL from request headers
			gatewayURL, err := getGatewayURL(req)
			if err != nil {
				reqLogger.Error("cannot determine gateway URL: %s", err)
				// Todo: NOTE Passing through was removed, please confirm ok
				http.Error(w, "Host header is required for agent card URL rewriting", http.StatusInternalServerError)
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
				reqLogger.Info("backend returned non-OK status: %d - passing through", rw.statusCode)
				return
			}

			// Validate content type
			contentType := rw.Header().Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				reqLogger.Warn("unexpected content-type: %s - passing through", contentType)
				return
			}

			// Parse agent card into map to preserve unknown fields
			var agentCardMap map[string]interface{}
			if err := json.Unmarshal(rw.body.Bytes(), &agentCardMap); err != nil {
				reqLogger.Error("failed to parse agent card: %s - passing through original", err)
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

			reqLogger.Info("transformed agent card URLs to external gateway format")

			// Write the transformed response
			w.Header().Set("Content-Type", "application/json")
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
