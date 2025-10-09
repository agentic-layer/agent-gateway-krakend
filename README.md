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

## Contribution

See [Contribution Guide](https://github.com/agentic-layer/agent-runtime-operator?tab=contributing-ov-file) for details on contribution, and the process for submitting pull requests.
