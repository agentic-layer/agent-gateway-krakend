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

### OpenAI-Compatible API

The gateway provides OpenAI-compatible endpoints (`/models` and `/chat/completions`) for agent access. For comprehensive documentation including request/response formats, configuration options, and protocol transformation details, see the [OpenAI to A2A Plugin README](go/plugin/openai-a2a/README.md).

**Quick Examples:**

List available agents:
```shell
curl http://localhost:10000/models
```

Send a chat completion request:
```shell
curl http://localhost:10000/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "local/mock-agent",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

For detailed API documentation, agent routing behavior, and model parameter formats, refer to the plugin documentation linked above.

### A2A via JSON-RPC

Verify the gateway proxy functionality with a JSON-RPC message:

```shell
curl http://localhost:10000/local/mock-agent \
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
       "metadata": {}
     }
   }' | jq
```


## Migration Guide

### Breaking Change: Removal of Per-Agent Chat Completions Endpoints

**Effective from version:** v0.5.0

The legacy per-agent chat completions endpoints have been removed in favor of the standard OpenAI-compatible global endpoint.

#### What Changed

**Old Endpoint Pattern (Removed):**
```
POST /{agent-name}/chat/completions
```

**New Endpoint Pattern:**
```
POST /chat/completions
```

#### Migration Steps

**Before:**
```shell
# Old endpoint - NO LONGER WORKS
curl http://localhost:10000/mock-agent/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Hello, how can you help me?"
      }
    ]
  }'
```

**After:**
```shell
# New endpoint - use model parameter to specify agent
curl http://localhost:10000/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "default/mock-agent",
    "messages": [
      {
        "role": "user",
        "content": "Hello, how can you help me?"
      }
    ]
  }'
```

#### Key Changes

1. **Endpoint URL**: Use `/chat/completions` instead of `/{agent-name}/chat/completions`
2. **Model Parameter**: The `model` field now specifies which agent to route to.
3. **Standardization**: The new endpoint follows the OpenAI API specification exactly

#### Benefits

- **OpenAI Compatibility**: Full compatibility with OpenAI client libraries and tools
- **Simplified API**: Single endpoint for all agents reduces API surface

## Contribution

See [Contribution Guide](https://github.com/agentic-layer/agent-gateway-krakend?tab=contributing-ov-file) for details on contribution, and the process for submitting pull requests.
