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

The `agentcard-rw` plugin rewrites Agent Card URLs to external gateway URLs and filters transport types. The plugin uses the `Host` header from incoming requests to construct the external gateway URL.

#### Quick Start with Docker Compose

The repository includes a pre-configured mock agent with test mappings:

```bash
# Start all services (gateway + mock agent + weather agent)
docker-compose up

# Wait for plugins to load (look for these logs):
# [AGENTCARD-RW  ] loaded
# [OPENAI-A2A    ] loaded
```

The mock agent automatically loads test data from `test/mappings/agent-card.json`, which includes various internal cluster URLs and transport types to demonstrate the rewriting functionality.

**Note:** You can customize the mock agent behavior by editing `test/mappings/agent-card.json`. See the [mock-agent configuration docs](https://github.com/agentic-layer/agent-samples/tree/main/wiremock/mock-agent#configuration) for details.

#### Test Comparison

**Direct to mock agent** (no rewriting):
```bash
curl http://localhost:8080/.well-known/agent-card.json | jq
# "http://localhost:8080"
```

**Through KrakenD gateway** (with agentcard-rw plugin):
```bash
curl -H "Host: gateway.agentic-layer.ai" \
       http://localhost:10000/mock-agent/.well-known/agent-card.json | jq
# "https://gateway.agentic-layer.ai/mock-agent"
```

#### What Gets Transformed

- ✅ **All URLs rewritten**: Agent endpoint URLs (`url` and `additionalInterfaces`) are always rewritten to gateway URLs
- ✅ **Transport filtering**: Only valid transports kept (JSONRPC, GRPC, HTTP+JSON - case-insensitive). Invalid transports removed (http, https, websocket, sse, etc.)
- ✅ **Provider URLs never rewritten**: Provider metadata remains unchanged

#### Full Response Example

<details>
<summary>Click to expand</summary>

**Before** (direct to mock agent):
```json
{
  "url": "http://localhost:8080",
  "additionalInterfaces": [
    {"transport": "JSONRPC", "url": "http://mock-agent:8080"},
    {"transport": "HTTP+JSON", "url": "http://10.96.1.50:8443"},
    {"transport": "grpc", "url": "grpc://mock-agent.default.svc.cluster.local:9090"}
  ]
}
```

**After** (through gateway):
```json
{
  "url": "https://gateway.agentic-layer.ai/mock-agent",
  "additionalInterfaces": [
    {"transport": "JSONRPC", "url": "https://gateway.agentic-layer.ai/mock-agent"},
    {"transport": "HTTP+JSON", "url": "https://gateway.agentic-layer.ai/mock-agent"},
    {"transport": "grpc", "url": "https://gateway.agentic-layer.ai/mock-agent"}
  ]
}
```
</details>

> **Note**: The plugin requires the `Host` header to be set in the request. In production, the ingress/load balancer automatically sets this header (e.g., `gateway.agentic-layer.ai`). For local testing, use the `-H "Host: ..."` flag with curl.

## Contribution

See [Contribution Guide](https://github.com/agentic-layer/agent-runtime-operator?tab=contributing-ov-file) for details on contribution, and the process for submitting pull requests.
