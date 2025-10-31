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

## Testing

### Setup

Start all services using Docker Compose:

```bash
docker compose up

# Wait for plugins to load (look for these logs):
# [AGENTCARD-RW  ] loaded
# [OPENAI-A2A    ] loaded
```

The repository includes a pre-configured mock agent that loads test data from `test/mappings/agent-card.json`. You can customize the mock agent behavior by editing this file. See the [mock-agent configuration docs](https://github.com/agentic-layer/agent-samples/tree/main/wiremock/mock-agent#configuration) for details.

### Test 1: Basic Gateway Connectivity

Verify the gateway proxy functionality with a JSON-RPC message:

```bash
curl http://localhost:8080/mock-agent \
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
             "text": "Hello, mock agent!"
           }
         ],
         "messageId": "9229e770-767c-417b-a0b0-f0741243c589",
         "contextId": "abcd1234-5678-90ab-cdef-1234567890ab"
       },
       "metadata": {"conversationId": "9229e770-767c-417b-a0b0-f0741243c589"}
     }
   }' | jq
```

### Test 2: Agent Card URL Rewriting

The `agentcard-rw` plugin rewrites Agent Card URLs to external gateway URLs and filters transport types based on the `Host` header.

**Direct to mock agent** (no rewriting):
```bash
curl http://localhost:8080/.well-known/agent-card.json | jq
# Returns: "url": "http://localhost:8080"
```

**Through KrakenD gateway** (with agentcard-rw plugin):
```bash
curl -H "Host: gateway.agentic-layer.ai" \
     http://localhost:10000/mock-agent/.well-known/agent-card.json | jq
# Returns: "url": "https://gateway.agentic-layer.ai/mock-agent"
```

**What gets transformed:**
- ✅ **All agent URLs rewritten**: Both `url` and `additionalInterfaces` are rewritten to gateway URLs
- ✅ **Transport filtering**: Only valid transports kept (JSONRPC, GRPC, HTTP+JSON). Invalid transports removed.
- ✅ **Provider URLs unchanged**: Provider metadata remains as-is

**Example transformation:**
```json
// Before (internal cluster URLs)
{"transport": "JSONRPC", "url": "http://mock-agent:8080"}
{"transport": "HTTP+JSON", "url": "http://10.96.1.50:8443"}

// After (gateway URLs)
{"transport": "JSONRPC", "url": "https://gateway.agentic-layer.ai/mock-agent"}
{"transport": "HTTP+JSON", "url": "https://gateway.agentic-layer.ai/mock-agent"}
```

> **Note**: The `Host` header is required for URL rewriting. In production, the ingress/load balancer sets this automatically. For local testing, use `-H "Host: gateway.agentic-layer.ai"` with curl.

### Test 3: Default Host Header Behavior

When no explicit `Host` header is provided, curl automatically sends `Host: localhost:10000`:

```bash
curl http://localhost:10000/mock-agent/.well-known/agent-card.json | jq 
```

**What's happening:**
- curl automatically sets `Host: localhost:10000` from the URL
- The plugin uses this to rewrite URLs, resulting in `localhost:10000` in the response

**Key takeaway:** To get production-like external URLs, explicitly set the Host header with `-H "Host: gateway.agentic-layer.ai"`.

## Contribution

See [Contribution Guide](https://github.com/agentic-layer/agent-runtime-operator?tab=contributing-ov-file) for details on contribution, and the process for submitting pull requests.
