## OpenAI to A2A Plugin

The OpenAI to A2A plugin provides OpenAI-compatible chat completion endpoints that automatically transform requests to the Agent-to-Agent (A2A) protocol format. This allows clients using the OpenAI API format to communicate with A2A-compatible agents.

### Features

- Intercepts requests at `/{path}/chat/completions` endpoints
- Transforms OpenAI chat completion format to A2A JSON-RPC 2.0 format
- Routes transformed requests to the corresponding agent endpoint at `/{path}`
- Automatically generates required A2A fields (messageId, contextId)
- Supports optional `X-Conversation-ID` header for conversation continuity
- Preserves authentication and other headers

### Request Flow

```
Client → /{agent-name}/chat/completions (OpenAI format)
         ↓ Plugin transformation
         → /{agent-name} (A2A JSON-RPC format)
         → Agent Backend
```

### OpenAI Request Format

```json
{
  "model": "gpt-4",
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

The endpoint suffix is `/chat/completions` by default, but can be configured:

```json
{
  "extra_config": {
    "plugin/http-server": {
      "name": [
        "openai-a2a"
      ],
      "openai_a2a_config": {
        "endpoint": "/chat/completion"
      }
    }
  }
}
```

### Example Usage

```bash
curl http://localhost:10000/mock-agent/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Conversation-ID: abcd1234-5678-90ab-cdef-1234567890ab" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "What is the weather in New York?"
      }
    ]
  }'
```

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