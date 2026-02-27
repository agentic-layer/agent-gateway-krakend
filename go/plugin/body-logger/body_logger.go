package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/agentic-layer/agent-gateway-krakend/lib/logging"
)

const pluginName = "body-logger"

type registerer string

// HandlerRegisterer is the symbol KrakenD looks up to register http-server plugins.
var HandlerRegisterer = registerer(pluginName)

var logger = logging.New(pluginName)

func main() {}

func init() {
	logger.Info("loaded")
}

type config struct {
	SkipPaths []string `json:"skip_paths"`
}

func parseConfig(extra map[string]interface{}) (config, error) {
	var cfg config
	raw, ok := extra["body_logger_config"]
	if !ok {
		return cfg, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return cfg, fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return cfg, nil
}

func (r registerer) RegisterHandlers(f func(
	name string,
	handler func(context.Context, map[string]interface{}, http.Handler) (http.Handler, error),
)) {
	f(string(r), r.registerHandlers)
	logger.Info("registered")
}

func (r registerer) RegisterLogger(v interface{}) {
	if kl, ok := logging.Wrap(v, pluginName); ok {
		logger = kl
	}
	logger.Info("logger registered")
}

func (r registerer) registerHandlers(_ context.Context, extra map[string]interface{}, handler http.Handler) (http.Handler, error) {
	cfg, err := parseConfig(extra)
	if err != nil {
		logger.Warning(fmt.Sprintf("failed to parse config, continuing without skip_paths: %v", err))
	}

	skipPaths := make(map[string]bool, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skipPaths[p] = true
	}

	logger.Info("plugin initialized successfully")
	return http.HandlerFunc(r.handleRequest(handler, skipPaths)), nil
}

func (r registerer) handleRequest(handler http.Handler, skipPaths map[string]bool) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if skipPaths[req.URL.Path] {
			handler.ServeHTTP(w, req)
			return
		}

		// Log request body
		if req.Body != nil {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				logger.Error("failed to read request body:", err)
			} else {
				if len(body) > 0 {
					logger.Debug(fmt.Sprintf("request [%s %s]:\n%s", req.Method, req.URL.Path, string(body)))
				}
				req.Body = io.NopCloser(bytes.NewReader(body))
			}
		}

		// Capture response to log it
		rw := &captureWriter{ResponseWriter: w, body: &bytes.Buffer{}, statusCode: http.StatusOK}
		handler.ServeHTTP(rw, req)

		// Log response body
		if rw.body.Len() > 0 {
			logger.Debug(fmt.Sprintf("response [%s %s] status=%d:\n%s", req.Method, req.URL.Path, rw.statusCode, rw.body.String()))
		}

		// Flush captured response to actual writer
		w.WriteHeader(rw.statusCode)
		if _, err := w.Write(rw.body.Bytes()); err != nil {
			logger.Error("failed to write response:", err)
		}
	}
}

type captureWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (cw *captureWriter) Write(b []byte) (int, error) {
	return cw.body.Write(b)
}

func (cw *captureWriter) WriteHeader(statusCode int) {
	cw.statusCode = statusCode
}
