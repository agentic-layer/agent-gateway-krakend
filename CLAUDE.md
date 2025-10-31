# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.
For more detailed information, refer to the [README.md](./README.md) file.

## Project Overview

This repository contains a KrakenD-based Agent Gateway implementation. It serves as an egress API gateway designed to route incoming requests to exposed agents within the agentic platform.

## Architecture

- **KrakenD Gateway**: Core API gateway service for routing agent requests
- **Agent Runtime Integration**: Works with the Agent Runtime Operator for agent discovery and routing
- **Kubernetes Deployment**: Deployed as AgentGateway custom resource in Kubernetes

## Development Setup

```shell
# Install system dependencies
brew bundle

# Build plugins
cd go
make agentcard-rw  # Agent Card URL Rewriting plugin
cd ..

# Build Docker image
docker build -t agentic-layer/agent-gateway-krakend .
```

## Testing with Mock Agent

The repository uses WireMock for testing. Mappings are in `test/mappings/`:

**Adding a new test endpoint:**
1. Create a `.json` file in `test/mappings/` (see existing files as examples)
2. Define request pattern:
   ```json
   "request": {
     "method": "POST",
     "bodyPatterns": [{"matchesJsonPath": "$.jsonrpc"}]
   }
   ```
3. Define response with optional templates:
   ```json
   "response": {
     "jsonBody": {"id": "{{jsonPath request.body '$.id'}}"},
     "transformers": ["response-template"]
   }
   ```
4. Restart mock-agent container: `docker-compose restart mock-agent`

## Agent Card URL Rewriting

The `agentcard-rw` plugin rewrites internal agent URLs to external gateway URLs.

**Valid Transport Protocols (case-insensitive):**
- `JSONRPC`
- `GRPC`
- `HTTP+JSON`

Any other transport in `additionalInterfaces` is silently filtered out.

**Requirements:**
- `Host` header must be present in requests for URL rewriting
- Local testing: use `-H "Host: gateway.agentic-layer.ai"` with curl
- Provider URLs are never modified (only agent endpoint URLs)

## Important Gotchas

- **Plugin handler order**: In `local/krakend.json`, the `extra_config.plugin/http-server.name` array defines handler order. Last entry = outermost/first handler.
- **WireMock reload**: Container must restart to pick up new mapping files
- **Transport filtering**: Invalid transports are removed silently without warnings

## Deployment Structure

The deployment follows the standard pattern:

```
deploy/
├── base/
│   ├── agent-gateway/        # Agent gateway deployment
│   │   ├── deployment.yaml   # AgentGateway custom resource
│   │   └── kustomization.yaml
│   ├── namespace.yaml        # Kubernetes namespace
│   └── kustomization.yaml    # Base resources
└── local/
    └── kustomization.yaml    # Local development overlay
```

## Dependencies

- **KrakenD**: API Gateway engine
- **Agent Runtime Operator**: Required for agent discovery and routing
- **Docker**: For containerization and deployment
- **Kubernetes**: For orchestration and deployment