# KrakenD Agent Gateway

A [KrakenD](https://www.krakend.io/docs/ai-gateway/) based Agent Gateway implementation that serves as an egress API gateway for routing incoming requests to exposed agents within the agentic platform.

----

## Table of Contents

- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Deployment](#deployment)
- [Architecture](#architecture)

----

## Prerequisites

The following tools and dependencies are required to run this project:

- **Docker**: For containerization and local development
- **Kubernetes**: For deployment and orchestration
- **kubectl**: Kubernetes command-line tool
- **Agent Runtime Operator**: Required for agent discovery and routing

----

## Getting Started

### 1. Install Dependencies

```bash
# Install system dependencies via Homebrew
brew bundle --no-lock --verbose
```

### 2. Build and Run Locally

```bash
# Build the Docker image
docker build -t agentic-layer/agent-gateway-krakend .

# Run the container
docker run -p 8080:8080 agentic-layer/agent-gateway-krakend
```

----

## Deployment

### Prerequisites for Deployment

**Important:** Ensure the Agent Runtime Operator is installed and running in your Kubernetes cluster before proceeding. For detailed setup instructions, please refer to the [Agent Runtime Operator Getting Started guide](https://github.com/agentic-layer/agent-runtime-operator?tab=readme-ov-file#getting-started).

### Deploy to Kubernetes

```bash
# Create required secrets for demo agent
kubectl create secret generic api-key-secret --from-literal=GOOGLE_API_KEY=$GOOGLE_API_KEY

# Deploy the agent gateway
kubectl apply -k deploy/local/
```

### Testing the Gateway

Test the proxy functionality with the following curl command:

```bash
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

----

## Architecture

The Agent Gateway is built using KrakenD and integrates with the Agent Runtime Operator to provide:

- **Request Routing**: Routes incoming requests to appropriate exposed agents
- **Agent Discovery**: Automatically discovers agents through the Agent Runtime Operator
- **Load Balancing**: Distributes requests across available agent instances
- **Protocol Support**: Handles Agent-to-Agent (A2A) communication protocols

The deployment structure follows standard Kubernetes patterns with base configurations and environment-specific overlays.

----

## License

This software is provided under the Apache v2.0 open source license, read the `LICENSE` file for details.
