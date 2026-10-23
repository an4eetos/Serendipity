---
description: Go transactions and domain event publishing
globs: backend/**/usecase/**/*.go,backend/**/service/**/*.go
alwaysApply: false
---

# Transactions & Events

- Every write operation executes inside TxManager.
- Never begin transactions manually inside adapters.
- Publish events only after successful commit.
- Publish events only via EventPublisher port.
- Never publish directly from adapters.
