---
description: Go domain and adapter error handling
globs: backend/**/domain/**/*.go,backend/**/adapters/**/*.go
alwaysApply: false
---

# Errors

Domain
- implement DomainError
- use typed errors
- constant messages

Forbidden
- fmt.Errorf
- transport errors
- SQL errors
- HTTP status codes

Adapters translate domain errors into transport-specific errors.
