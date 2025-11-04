# Agent Gateway KrakenD

A [KrakenD](https://www.krakend.io/docs/) based Agent Gateway implementation that serves as an egress API gateway for routing incoming requests to exposed agents within the agentic platform.

## Plugins

Following plugins are included in this repository:

- [Agent Card URL Rewriting Plugin](go/plugin/agentcard-rw/README.md)
- [OpenAI A2A Plugin](go/plugin/openai-a2a/README.md)


## Development

### Prerequisites

The following tools are required for development:

- **Docker**: For containerization and local development

### Building Plugins

```shell
make plugins
```

This will compile the plugins and output it to `build/`.

### Run with Docker Compose

Start the agent gateway using Docker Compose:

```shell
# Then start the services
docker compose up --build
```

Stop the services:

```shell
docker compose down
```

## Testing

### A2A via JSON-RPC

Verify the gateway proxy functionality with a JSON-RPC message:

```shell
curl http://localhost:10000/mock-agent \
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


## Contribution

See [Contribution Guide](https://github.com/agentic-layer/agent-gateway-krakend?tab=contributing-ov-file) for details on contribution, and the process for submitting pull requests.
