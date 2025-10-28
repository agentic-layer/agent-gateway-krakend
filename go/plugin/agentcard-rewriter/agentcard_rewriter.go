package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/agentic-layer/agent-gateway-krakend/lib/logging"
	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
)

const (
	pluginName = "agentcard-rw"
	configKey  = "agentcard_rw_config"
)

type config struct {
	GatewayDomain string `json:"gateway_domain"` // e.g., "https://gateway.agentic-layer.ai"
	PathPrefix    string `json:"path_prefix"`    // e.g., "/agents" (optional, defaults to "")
}

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

// parseConfig parses plugin configuration from KrakenD extra_config
func parseConfig(extra map[string]interface{}, cfg *config) error {
	// If no config provided, return empty config (use header auto-detection only)
	if extra[configKey] == nil {
		logger.Info("no configuration provided, using header auto-detection only")
		return nil
	}

	pluginConfig, ok := extra[configKey].(map[string]interface{})
	if !ok {
		return fmt.Errorf("cannot read extra_config.%s", configKey)
	}

	raw, err := json.Marshal(pluginConfig)
	if err != nil {
		return fmt.Errorf("cannot marshal extra config back to JSON: %s", err.Error())
	}

	err = json.Unmarshal(raw, cfg)
	if err != nil {
		return fmt.Errorf("cannot parse extra config: %s", err.Error())
	}

	logger.Info("configuration loaded: gateway_domain=%s, path_prefix=%s", cfg.GatewayDomain, cfg.PathPrefix)
	return nil
}

func (r registerer) registerHandlers(_ context.Context, extra map[string]interface{}, handler http.Handler) (http.Handler, error) {
	var cfg config
	err := parseConfig(extra, &cfg)
	if err != nil {
		return nil, err
	}
	logger.Info("configuration loaded successfully")
	return http.HandlerFunc(r.handleRequest(cfg, handler)), nil
}

func (r registerer) handleRequest(cfg config, handler http.Handler) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		reqLogger := logging.NewWithPluginName(pluginName)

		// Check if this is a GET request to an agent card endpoint
		if req.Method == http.MethodGet && isAgentCardEndpoint(req.URL.Path) {
			reqLogger.Info("intercepted agent card request: %s", req.URL.Path)

			// Get gateway domain from request headers (with config fallback)
			gatewayDomain, err := getGatewayDomain(req, cfg)
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
				copyHeaders(w.Header(), rw.Header())
				w.WriteHeader(rw.statusCode)
				w.Write(rw.body.Bytes())
				return
			}

			// Validate content type
			contentType := rw.Header().Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				reqLogger.Warn("unexpected content-type: %s - passing through", contentType)
				copyHeaders(w.Header(), rw.Header())
				w.WriteHeader(rw.statusCode)
				w.Write(rw.body.Bytes())
				return
			}

			// Parse agent card
			var agentCard models.AgentCard
			if err := json.Unmarshal(rw.body.Bytes(), &agentCard); err != nil {
				reqLogger.Error("failed to parse agent card: %s - passing through original", err)
				copyHeaders(w.Header(), rw.Header())
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
			agentCard = rewriteAgentCard(agentCard, gatewayDomain, agentName, cfg.PathPrefix)

			// Marshal rewritten agent card
			rewrittenBody, err := json.Marshal(agentCard)
			if err != nil {
				reqLogger.Error("failed to marshal rewritten agent card: %s", err)
				http.Error(w, "failed to create rewritten agent card", http.StatusInternalServerError)
				return
			}

			reqLogger.Info("transformed agent card URLs to external gateway format")

			// Todo remove before merge
			// Log the rewritten body for debugging
			bodyPreview := string(rewrittenBody)
			if len(bodyPreview) > 200 {
				bodyPreview = bodyPreview[:200] + "..."
			}
			reqLogger.Info("rewritten body size: %d bytes, preview: %s", len(rewrittenBody), bodyPreview)

			// Write the transformed response (following openai-a2a plugin pattern)
			w.Header().Set("Content-Type", "application/json")
			// Remove Content-Length to allow for recalculation
			w.Header().Del("Content-Length")
			w.WriteHeader(http.StatusOK)

			reqLogger.Info("about to write body (%d bytes)", len(rewrittenBody))
			bytesWritten, err := w.Write(rewrittenBody)
			if err != nil {
				reqLogger.Error("FAILED to write response: %s", err)
			} else {
				reqLogger.Info("successfully wrote %d bytes to response", bytesWritten)
			}
			return
		}

		// Not an agent card endpoint, pass through
		handler.ServeHTTP(w, req)
	}
}
