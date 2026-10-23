---
description: Go infrastructure wiring and DI rules
globs: backend/infra/**/*.go
alwaysApply: false
---

# Infrastructure

Infrastructure wires the application.

Allowed
- configuration
- dependency injection
- http clients
- cron
- workers
- database initialization
- observability

Forbidden
- business rules
