# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.
For more detailed information, refer to the [README.md](./README.md) file.

## Project Overview

This repository contains a KrakenD-based Agent Gateway implementation. It serves as an egress API gateway designed to route incoming requests to exposed agents within the agentic platform.

## Architecture

- **KrakenD Gateway**: Core API gateway service for routing agent requests
- **Agent Runtime Integration**: Works with the Agent Runtime Operator for agent discovery and routing
- **Kubernetes Deployment**: Deployed as AgentGateway custom resource in Kubernetes

## Development Setup

```shell
# Install system dependencies
brew bundle

# Build Docker image
docker build -t agentic-layer/agent-gateway-krakend .
```

## Deployment Structure

The deployment follows the standard pattern:

```
deploy/
├── base/
│   ├── agent-gateway/        # Agent gateway deployment
│   │   ├── deployment.yaml   # AgentGateway custom resource
│   │   └── kustomization.yaml
│   ├── namespace.yaml        # Kubernetes namespace
│   └── kustomization.yaml    # Base resources
└── local/
    └── kustomization.yaml    # Local development overlay
```

## Dependencies

- **KrakenD**: API Gateway engine
- **Agent Runtime Operator**: Required for agent discovery and routing
- **Docker**: For containerization and deployment
- **Kubernetes**: For orchestration and deployment