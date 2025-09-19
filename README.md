# KrakenD based Agent Gateway

This is a [KrakenD](https://www.krakend.io/docs/ai-gateway/) based Agent Gateway implementation. This is an egress API gateway meant to route incoming requests to the exposed agents.

### Building and Running

```bash
# install local development dependencies
brew bundle --no-lock --verbose

# Build the Docker image
docker build -t agentic-layer/agent-gateway-krakend .

# Run the container
docker run -p 8080:8080 agentic-layer/agent-gateway-krakend
```

## Deployment

**Note:** Ensure the Agent Runtime Operator is installed and running in your Kubernetes cluster before proceeding.
For detailed setup instructions, please refer to the [Agent Runtime Operator Getting Started guide](https://github.com/agentic-layer/agent-runtime-operator?tab=readme-ov-file#getting-started).

```bash
# in order for the demo agent to work we have to manually create a Kubernetes secrets
kubectl create secret generic api-key-secret --from-literal=GOOGLE_API_KEY=$GOOGLE_API_KEY
kubectl apply -k kustomize/local/

# to test the proxy, issue the following curl command
curl http://krakend.127.0.0.1.sslip.io/weather_agent/a2a/ \
  -H "Content-Type: application/json" \
  -d '{
     "jsonrpc": "2.0",
     "id": 1,
     "method": "message/send",
     "params": {
       "message": {
         "role": "agent",
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

## License

This software is provided under the Apache v2.0 open source license, read the `LICENSE` file for details.
