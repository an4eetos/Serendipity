---
description: Go backend screaming architecture and layer boundaries
globs: backend/**/*.go
alwaysApply: false
---

# Architecture

## General

- Use Screaming Architecture.
- Folder names represent business capabilities, not technologies.
- Keep dependencies acyclic.
- Maximum file size: 800 lines.
- Prefer composition over inheritance.
- Prefer constructor injection.
- Keep packages cohesive.

## Business Logic

Business logic is allowed only inside:
- domain
- service
- usecase

Business logic MUST NOT exist in:
- cmd
- infra
- adapters
- handlers
- converters
- middleware
