---
description: Go backend dependency flow and import rules
globs: backend/**/*.go
alwaysApply: false
---

# Dependency Rules

Allowed dependency flow

cmd
↓
infra
↓
driving adapters
↓
usecases
↓
domain
↑
ports
↑
driven adapters

Rules

- Domain imports nothing except standard library.
- Domain never imports adapters.
- Domain never imports infra.
- Domain never imports pkg.
- Services depend only on domain.
- Usecases depend only on ports.
- Adapters implement ports.
- Infra wires everything together.
- pkg never imports internal.
