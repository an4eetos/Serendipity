---
description: Go use case orchestration rules
globs: backend/**/usecase/**/*.go
alwaysApply: false
---

# Usecases

Responsibilities
- orchestrate business workflows
- coordinate multiple ports
- call domain services
- start and finish transactions
- publish events after successful commits

Rules
- depend only on interfaces
- never depend directly on adapters
- one usecase = one business action
- keep orchestration readable
