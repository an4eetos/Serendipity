---
description: Go testing strategy by layer
globs: backend/**/*_test.go
alwaysApply: false
---

# Testing

Domain
- unit tests

Services
- unit tests

Usecases
- mock ports

Adapters
- integration tests

Use lightweight Docker services for PostgreSQL, Redis, Kafka, MinIO where appropriate.
