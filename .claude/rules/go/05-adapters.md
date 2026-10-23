---
description: Go driving and driven adapter boundaries
globs: backend/**/adapters/**/*.go
alwaysApply: false
---

# Driving Adapters

Examples
- HTTP
- gRPC
- CLI
- Kafka consumers

Responsibilities
- parse requests
- validate transport payloads
- call usecases
- convert responses

Forbidden
- business decisions
- transaction handling
- persistence logic

# Driven Adapters

Examples
- PostgreSQL
- Redis
- S3
- Kafka producer

Responsibilities
- implement ports
- communicate with external systems

Forbidden
- business logic
- event orchestration
