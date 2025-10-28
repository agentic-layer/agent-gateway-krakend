# Agent Gateway KrakenD

A [KrakenD](https://www.krakend.io/docs/ai-gateway/) based Agent Gateway implementation that serves as an egress API gateway for routing incoming requests to exposed agents within the agentic platform.

## Development

### Prerequisites

The following tools are required for development:

- **Docker**: For containerization and local development

### Build and Deploy

#### Building Plugins

Build the openai-a2a plugin:

```bash
cd go
make openai-a2a
```

This will compile the plugin and output it to `build/openai-a2a.so`.

#### Docker Compose

Start the agent gateway using Docker Compose:

```bash
# Copy environment template
cp .env.example .env

# Edit .env with your configuration
# Then start the services
docker-compose up --build
```

Stop the services:

```bash
docker-compose down
```

#### Manual Docker Build

Build the Docker image:

```bash
docker build -t agentic-layer/agent-gateway-krakend .
```

Run the container locally:

```bash
docker run -p 8080:8080 -v $(pwd)/local/krakend.json:/etc/krakend/krakend.json:ro agentic-layer/agent-gateway-krakend
```

### Testing the Gateway

Test the proxy functionality:

```bash
curl http://localhost:8080/weather-agent \
  -H "Content-Type: application/json" \
  -d '{
     "jsonrpc": "2.0",
     "id": 1,
     "method": "message/send",
     "params": {
       "message": {
         "role": "user",
         "parts": [
           {
             "kind": "text",
             "text": "What is the weather in New York?"
           }
         ],
         "messageId": "9229e770-767c-417b-a0b0-f0741243c589",
         "contextId": "abcd1234-5678-90ab-cdef-1234567890ab"
       },
       "metadata": {"conversationId": "9229e770-767c-417b-a0b0-f0741243c589"}
     }
   }' | jq
```

### Testing Agent Card URL Rewriting

The `agentcard-rw` plugin rewrites internal Kubernetes cluster URLs to external gateway URLs and filters out unsupported transports.

#### Prerequisites

Start the mock agent from `agent-samples`:

```bash
cd ./PAAL/agent-samples/wiremock/mock-agent
make docker-run  # Starts on port 8080
```

Configure the mock agent with test data (internal cluster URLs):

```bash
curl -X POST http://localhost:8080/__admin/mappings \
  -H 'Content-Type: application/json' \
  -d '{
    "request": {
      "method": "GET",
      "urlPath": "/.well-known/agent-card.json"
    },
    "response": {
      "status": 200,
      "headers": {"Content-Type": "application/json"},
      "jsonBody": {
        "name": "mock_agent",
        "description": "Mock Agent that echoes back the input text 1:1",
        "preferredTransport": "JSONRPC",
        "protocolVersion": "0.3.0",
        "capabilities": {},
        "supportsAuthenticatedExtendedCard": false,
        "url": "http://mock-agent.default.svc.cluster.local:8080",
        "additionalInterfaces": [
          {"transport": "http", "url": "http://mock-agent.default.svc.cluster.local:8080"},
          {"transport": "https", "url": "https://mock-agent.default.svc.cluster.local:8443"},
          {"transport": "grpc", "url": "grpc://mock-agent.default.svc.cluster.local:9090"},
          {"transport": "websocket", "url": "ws://mock-agent.default.svc.cluster.local:8080/ws"},
          {"transport": "http", "url": "https://external-service.example.com/api"}
        ],
        "provider": {
          "name": "Test Provider",
          "url": "https://test-provider.example.com"
        },
        "version": "0.0.1"
      }
    }
  }'
```

Start the KrakenD gateway:

```bash
cd /path/to/agent-gateway-krakend

# Option 1: Using Docker Compose
docker-compose up --build

# Option 2: Using Docker directly
docker build -t agentic-layer/agent-gateway-krakend .
docker run -p 10000:10000 \
  -v $(pwd)/local/krakend.json:/etc/krakend/krakend.json:ro \
  agentic-layer/agent-gateway-krakend
```

Wait for the plugins to load (look for these logs):
```
[AGENTCARD-RW  ] loaded
[OPENAI-A2A    ] loaded
```

#### Test Comparison

**Direct to mock agent** (no rewriting):
```bash
curl http://localhost:8080/.well-known/agent-card.json | jq .url
# "http://mock-agent.default.svc.cluster.local:8080"
```

**Through KrakenD gateway** (with agentcard-rw plugin):
```bash
curl -H "Host: gateway.agentic-layer.ai" \
       http://localhost:10000/mock-agent/.well-known/agent-card.json | jq| jq .url
# "https://gateway.agentic-layer.ai/mock-agent"
```

#### What Gets Transformed

- ✅ Internal cluster URLs (`*.svc.cluster.local`) → Gateway URLs
- ✅ Transport filtering: Only HTTP/HTTPS kept, gRPC/WebSocket/SSE removed
- ✅ External URLs preserved unchanged
- ✅ Provider URLs never rewritten

#### Full Response Example

<details>
<summary>Click to expand</summary>

**Before** (direct to mock agent):
```json
{
  "url": "http://mock-agent.default.svc.cluster.local:8080",
  "additionalInterfaces": [
    {"transport": "http", "url": "http://mock-agent.default.svc.cluster.local:8080"},
    {"transport": "https", "url": "https://mock-agent.default.svc.cluster.local:8443"},
    {"transport": "grpc", "url": "grpc://mock-agent.default.svc.cluster.local:9090"},
    {"transport": "websocket", "url": "ws://mock-agent.default.svc.cluster.local:8080/ws"},
    {"transport": "http", "url": "https://external-service.example.com/api"}
  ]
}
```

**After** (through gateway):
```json
{
  "url": "https://gateway.agentic-layer.ai/mock-agent",
  "additionalInterfaces": [
    {"transport": "http", "url": "https://gateway.agentic-layer.ai/mock-agent"},
    {"transport": "https", "url": "https://gateway.agentic-layer.ai/mock-agent"},
    {"transport": "http", "url": "https://external-service.example.com/api"}
  ]
}
```
</details>

> **Note**: In production, the ingress/load balancer sets the `Host` header (e.g., `gateway.agentic-layer.ai`), which takes precedence over the configured `gateway_url`. The config provides a fallback for local testing.

## Contribution

See [Contribution Guide](https://github.com/agentic-layer/agent-runtime-operator?tab=contributing-ov-file) for details on contribution, and the process for submitting pull requests.
