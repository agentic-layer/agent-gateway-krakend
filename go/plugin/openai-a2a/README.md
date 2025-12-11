## OpenAI to A2A Plugin

The OpenAI to A2A plugin provides OpenAI-compatible chat completion endpoints that automatically transform requests to the Agent-to-Agent (A2A) protocol format. This allows clients using the OpenAI API format to communicate with A2A-compatible agents.

### Features

- **Global `/chat/completions` endpoint**: Single endpoint for all agents using model-based routing
- **`/models` endpoint**: List all available agents as OpenAI-compatible models
- **Protocol transformation**: Converts OpenAI format to A2A JSON-RPC 2.0 format
- **Dynamic routing**: Routes requests to agents based on the `model` parameter
- **Auto-generation**: Automatically generates required A2A fields (messageId, contextId)
- **Flexible identifiers**: Supports any model ID format provided by configuration
- **Conversation continuity**: Supports optional `X-Conversation-ID` header for maintaining context across requests

### Request Flow

```
Client → /chat/completions (OpenAI format)
         ↓ Plugin parses model parameter and resolves agent
         → /{model-id} (A2A JSON-RPC format)
         → Agent Backend
```

### OpenAI Request Format

```json
{
  "model": "default/weather-agent",
  "messages": [
    {
      "role": "user",
      "content": "What is the weather in New York?"
    }
  ],
  "temperature": 0.7
}
```

### Transformed A2A Request

```json
{
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
    "metadata": {}
  }
}
```

### Configuration

The plugin is configured via `openai_a2a_config` in the KrakenD configuration. The agents list is populated by the configuration manager (e.g., the Kubernetes operator in a K8s deployment).

```json
{
  "extra_config": {
    "plugin/http-server": {
      "name": [
        "openai-a2a"
      ],
      "openai_a2a_config": {
        "agents": [
          {
            "model_id": "default/weather-agent",
            "url": "http://weather-agent:8000",
            "owned_by": "default",
            "createdAt": 1731679815
          }
        ]
      }
    }
  }
}
```

### Example Usage

#### List Available Models

```bash
curl http://localhost:10000/models
```

Response:
```json
{
  "object": "list",
  "data": [
    {
      "id": "local/mock-agent",
      "object": "model",
      "created": 1731679815,
      "owned_by": "local"
    }
  ]
}
```

#### Send Chat Completion Request

```bash
curl http://localhost:10000/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Conversation-ID: abcd1234-5678-90ab-cdef-1234567890ab" \
  -d '{
    "model": "local/mock-agent",
    "messages": [
      {
        "role": "user",
        "content": "What is the weather in New York?"
      }
    ]
  }'
```

**Model Parameter:**

The `model` field specifies which agent to route to. Model ID format is determined by your gateway configuration.

### Conversation ID Management

The plugin supports conversation continuity through the `X-Conversation-ID` header:

- **With header**: When the `X-Conversation-ID` header is provided, its value is used as the `contextId` in the A2A message, enabling conversation continuity across multiple requests
- **Without header**: If no header is provided, a new UUID is automatically generated for the `contextId`, and a warning is logged

This allows clients to maintain conversation context by sending the same conversation ID across related requests.

### Message Handling

The plugin uses the last user message from the OpenAI messages array as the primary message content for the A2A request. All other messages (system messages and earlier conversation context) are forwarded to the history field in the A2A transformation.

### Protocol References

- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat)
- [A2A Protocol Specification](https://a2a-protocol.org/latest/specification/)