---
description: Go domain layer — pure business rules only
globs: backend/**/domain/**/*.go
alwaysApply: false
---

# Domain

The domain contains pure business rules.

Allowed:
- entities
- value objects
- domain services
- domain errors
- validation
- business invariants

Forbidden:
- SQL
- Redis
- HTTP
- Kafka
- gRPC
- logging
- metrics
- transactions
- configuration
- filesystem
- environment variables
