package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/agentic-layer/agent-gateway-krakend/lib/logging"
	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
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
	// No configuration needed - plugin auto-detects gateway domain from request headers
	return http.HandlerFunc(r.handleRequest(handler)), nil
}

func (r registerer) handleRequest(handler http.Handler) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		reqLogger := logging.NewWithPluginName(pluginName)

		// Check if this is a GET request to an agent card endpoint
		if req.Method == http.MethodGet && isAgentCardEndpoint(req.URL.Path) {
			reqLogger.Info("intercepted agent card request: %s", req.URL.Path)

			// Get gateway domain from request headers
			gatewayDomain, err := getGatewayDomain(req)
			if err != nil {
				reqLogger.Warn("cannot determine gateway domain: %s - passing through", err)
				handler.ServeHTTP(w, req)
				return
			}

			// Extract agent name from path
			agentName := extractAgentName(req.URL.Path)
			if agentName == "" {
				reqLogger.Warn("cannot extract agent name from path: %s - passing through", req.URL.Path)
				handler.ServeHTTP(w, req)
				return
			}

			reqLogger.Info("rewriting URLs for agent: %s, gateway: %s", agentName, gatewayDomain)

			// Wrap response writer to capture backend response
			rw := newResponseWriter(w)

			// Forward request to backend
			handler.ServeHTTP(rw, req)

			// Only transform successful responses
			if rw.statusCode != http.StatusOK {
				reqLogger.Info("backend returned non-OK status: %d - passing through", rw.statusCode)
				w.WriteHeader(rw.statusCode)
				w.Write(rw.body.Bytes())
				return
			}

			// Validate content type
			contentType := rw.Header().Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				reqLogger.Warn("unexpected content-type: %s - passing through", contentType)
				w.WriteHeader(rw.statusCode)
				w.Write(rw.body.Bytes())
				return
			}

			// Parse agent card
			var agentCard models.AgentCard
			if err := json.Unmarshal(rw.body.Bytes(), &agentCard); err != nil {
				reqLogger.Error("failed to parse agent card: %s - passing through original", err)
				w.WriteHeader(rw.statusCode)
				w.Write(rw.body.Bytes())
				return
			}

			// Check provider URL for suspicious internal URLs
			if agentCard.Provider != nil && agentCard.Provider.Url != "" {
				if shouldWarn, reason := checkProviderURL(agentCard.Provider.Url); shouldWarn {
					reqLogger.Warn("provider.url contains internal URL: %s (%s) - not rewriting but this may be incorrect",
						agentCard.Provider.Url, reason)
				}
			}

			// Rewrite agent card URLs
			agentCard = rewriteAgentCard(agentCard, gatewayDomain, agentName)

			// Marshal rewritten agent card
			rewrittenBody, err := json.Marshal(agentCard)
			if err != nil {
				reqLogger.Error("failed to marshal rewritten agent card: %s", err)
				http.Error(w, "failed to create rewritten agent card", http.StatusInternalServerError)
				return
			}

			reqLogger.Info("transformed agent card URLs to external gateway format")

			// Write the transformed response
			w.Header().Set("Content-Type", "application/json")
			w.Header().Del("Content-Length") // Allow recalculation
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
