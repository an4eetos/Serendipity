---
description: Go configuration loading and injection
globs: backend/infra/config/**/*.go
alwaysApply: false
---

# Configuration

- Strongly typed config structs.
- Parse configuration only in infra.
- Inject config via constructors.

Forbidden
- Viper
- global config
- environment lookups outside infra
