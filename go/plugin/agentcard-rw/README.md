# agentcard-rw Plugin

Replace the internal URL in agent cards with the external gateway URL, when agent cards are proxied.

## Testing

The repository includes a pre-configured mock agent that loads test data from [agent-card.json](../../../local/wiremock/mappings/agent-card.json). You can customize the mock agent behavior by editing this file. See the [mock-agent configuration docs](https://github.com/agentic-layer/agent-samples/tree/main/wiremock/mock-agent#configuration) for details.

### Retrieve Agent Card

```shell
curl http://localhost:10000/mock-agent/.well-known/agent-card.json | jq
```

The `agentcard-rw` plugin rewrites URLs in the Agent Card to external gateway URLs (in this case http://localhost:10000).
This effects the default URL and the additional interfaces. Only known transport types are included.
